package lang

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bake/internal/lang/values"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type Task struct {
	Name        string
	Description string   `hcl:"description,optional"`
	Command     string   `hcl:"command,optional"`
	Creates     string   `hcl:"creates,optional"`
	Sources     []string `hcl:"sources,optional"`
	Remain      hcl.Body `hcl:",remain"`
	ExitCode    values.EventualInt64
	metadata    TaskMetadata
}

type TaskMetadata struct {
	// manual metadata
	Block hcl.Range
	// metadata from block
	// Description cannot be fetch from Block since it was already decoded
	Command   hcl.Range
	Creates   hcl.Range
	Sources   hcl.Range
	Remain    hcl.Range
	DependsOn hcl.Range
}

func NewTask(raw addressBlock, ctx *hcl.EvalContext) (*Task, hcl.Diagnostics) {
	meta := TaskMetadata{Block: raw.Block.DefRange}
	diags := DecodeRange(raw.Block.Body, ctx, &meta)
	if diags.HasErrors() {
		return nil, diags
	}

	task := &Task{Name: raw.GetName(), metadata: meta}
	diags = gohcl.DecodeBody(raw.Block.Body, ctx, task)
	if diags.HasErrors() {
		return nil, diags
	}

	diags = checkDependsOn(task.Remain)
	if diags.HasErrors() {
		return nil, diags
	}

	return task, nil
}

func (t Task) GetName() string {
	return t.Name
}

func (t Task) GetPath() cty.Path {
	// todo: change this to deal with for_each cases
	return cty.GetAttrPath(TaskLabel).GetAttr(t.GetName())
}

func (t Task) GetFilename() string {
	return t.metadata.Block.Filename
}

func (t Task) CTY() cty.Value {
	value := values.StructToCty(t)
	m := value.AsValueMap()
	m[NameLabel] = cty.StringVal(t.GetName())
	return cty.ObjectVal(m)
}

func (t *Task) Apply(state State) hcl.Diagnostics {
	// dont apply twice in case more than 1 task depends on this
	if t.ExitCode.Valid || t.Command == "" {
		return nil
	}

	log := state.NewLogger(t)
	if state.Prune {
		shouldRun, description, diags := t.dryPrune(state)
		if diags.HasErrors() {
			return diags
		}

		log.Println(description)
		if !shouldRun || state.Dry {
			return nil
		}

		return t.prune(log)
	}

	// run
	shouldRun, description, diags := t.dryRun(state)
	if diags.HasErrors() {
		return diags
	}

	log.Println(description)
	if !shouldRun || state.Dry {
		return nil
	}

	return t.run(log)
}

func (t Task) dryRun(state State) (shouldApply bool, reason string, diags hcl.Diagnostics) {
	if state.Force {
		return true, "force run is in effect", nil
	}

	if t.Command == "" && t.Creates != "" {
		return false, "", hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  `"command" cannot be empty when "creates" is provided`,
			Subject:  &t.metadata.Creates,
			Context:  &t.metadata.Block,
		}}
	}

	// phony task
	if len(t.Sources) == 0 || t.Creates == "" {
		return true, `"sources" or "creates" was not specified ... baking phony task`, nil
	}

	// if the target doesn't exist then it should be created
	targetInfo, err := os.Stat(t.Creates)
	if err != nil {
		return true, fmt.Sprintf(`"%s" doesn't exists ... baking`, t.Creates), nil
	}

	for _, pattern := range t.Sources {
		// Check pattern is well-formed.
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return false, "", hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf(`pattern "%s" is malformed`, pattern),
				Detail:   err.Error(),
				Subject:  &t.metadata.Sources,
				Context:  &t.metadata.Block,
			}}
		}

		if len(matches) == 0 {
			return false, fmt.Sprintf(`pattern "%s" doesn't match anything ... skipping`, strings.Join(t.Sources, ", ")), nil
		}

		for _, filename := range matches {
			info, err := os.Stat(filename)
			if err != nil {
				return false, "", hcl.Diagnostics{{
					Severity: hcl.DiagError,
					Summary:  fmt.Sprintf(`error getting "%s" stat information`, filename),
					Detail:   err.Error(),
					Subject:  &t.metadata.Sources,
					Context:  &t.metadata.Block,
				}}
			}

			// sources are newer than target, create it
			if info.ModTime().After(targetInfo.ModTime()) {
				return true, fmt.Sprintf(`source "%s" is newer than "%s" ... baking`, filename, t.Creates), nil
			}
		}
	}

	return false, fmt.Sprintf(`"%s" is newer than "%s" ... skipping`, t.Creates, strings.Join(t.Sources, "")), nil
}

func (t *Task) run(log *log.Logger) hcl.Diagnostics {
	// determine which shell to use
	terminal := "bash"
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		terminal = shell
	}

	// use fail fast flags
	script := fmt.Sprintf(`
	set -euo pipefail

	%s`, t.Command)
	// todo: use exec.CommandContext
	command := exec.Command(terminal, "-c", script)
	// todo: should this be configurable?
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	log.Println(`done in ` + command.ProcessState.UserTime().String())
	// store results
	t.ExitCode = values.EventualInt64{
		Int64: int64(command.ProcessState.ExitCode()),
		Valid: true,
	}

	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		detail = strings.TrimSpace(stdout.String())
	}

	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`"%s" task failed with exit code %d`, PathString(t.GetPath()), t.ExitCode.Int64),
			Detail:   detail,
			Subject:  &t.metadata.Command,
			Context:  &t.metadata.Block,
		}}
	}

	if t.Creates == "" {
		return nil
	}

	_, err = os.Stat(t.Creates)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`"%s" didn't create the expected file "%s"`, PathString(t.GetPath()), t.Creates),
			Subject:  &t.metadata.Creates,
			Context:  &t.metadata.Block,
		}}
	}

	return nil
}

func (t Task) dryPrune(state State) (shouldApply bool, reason string, diags hcl.Diagnostics) {
	if state.Force {
		return true, "force prunning is in effect", nil
	}

	if t.Creates == "" {
		return false, "nothing to prune", nil
	}

	stat, err := os.Stat(t.Creates)
	if err != nil {
		return false, fmt.Sprintf(`"%s" doesn't exist`, t.Creates), nil
	}

	return true, fmt.Sprintf(`will delete "%s"`, stat.Name()), nil
}

func (t *Task) prune(log *log.Logger) hcl.Diagnostics {
	err := os.RemoveAll(t.Creates)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "error pruning task " + t.GetName(),
			Detail:   err.Error(),
			Subject:  &t.metadata.Creates,
			Context:  &t.metadata.Block,
		}}
	}

	return nil
}

package lang

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"bake/internal/values"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type Task struct { // todo: what is really optional?
	addressBlock
	Description string   `hcl:"description,optional"`
	Command     string   `hcl:"command,optional"`
	Creates     string   `hcl:"creates,optional"`
	Sources     []string `hcl:"sources,optional"`
	Remain      hcl.Body `hcl:",remain"`
	exitCode    values.EventualInt64
}

func NewTask(raw addressBlock, ctx *hcl.EvalContext) (*Task, hcl.Diagnostics) {
	task := &Task{addressBlock: raw}
	diagnostics := gohcl.DecodeBody(raw.block.Body, ctx, task)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	diagnostics = checkDependsOn(task.Remain)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return task, nil
}

func (t Task) CTY() cty.Value {
	value := values.StructToCty(t)
	m := value.AsValueMap()
	m[NameLabel] = cty.StringVal(t.GetName())
	return cty.ObjectVal(m)
}

func (t Task) Plan() (shouldApply bool, reason string, diags hcl.Diagnostics) {
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
				Subject:  getSourcesRange(t.block),
				Context:  t.block.DefRange.Ptr(),
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
					Subject:  getSourcesRange(t.block),
					Context:  t.block.DefRange.Ptr(),
				}}
			}

			// sources are newer than target, create it
			if info.ModTime().After(targetInfo.ModTime()) {
				return true, fmt.Sprintf(`source "%s" is newer than "%s" ... baking`, filename, t.Creates), nil
			}
		}
	}

	return false, fmt.Sprintf(`no source matching "%s" is newer than "%s" ... skipping`, strings.Join(t.Sources, ""), t.Creates), nil
}

func (t *Task) Apply() hcl.Diagnostics {
	if t.exitCode.Valid {
		return nil
	}

	// log.Println("executing " + PathString(t.GetPath()))

	terminal := "bash"
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		terminal = shell
	}

	script := fmt.Sprintf(`
	set -euo pipefail

	%s`, t.Command)
	command := exec.Command(terminal, "-c", script)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	// todo: keep a ref to command.ProcessState since it contains useful info
	// like process time, exit code, etc
	t.exitCode = values.EventualInt64{
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
			Summary:  fmt.Sprintf(`"%s" task failed with exit code %d`, PathString(t.GetPath()), t.exitCode.Int64),
			Detail:   detail,
			Subject:  getCommandRange(t.block),
			Context:  t.block.DefRange.Ptr(),
		}}
	}

	return nil
}

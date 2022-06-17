package lang

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"bake/internal/lang/config"
	"bake/internal/lang/values"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/hashicorp/hcl/v2"
)

func (t Task) dryRun(state *config.State) (shouldApply bool, reason string, diags hcl.Diagnostics) {
	if state.Flags.Force {
		return true, "force run is in effect", nil
	}

	oldHash, ok := state.Lock.Tasks[AddressToString(t)]
	if ok {
		hash := t.Hash()
		if hash.Creates != oldHash.Creates {
			return true, `"creates" has changed ... baking`, nil
		}

		if hash.Command != oldHash.Command {
			return true, `"command" has changed ... baking`, nil
		}
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
		FS := os.DirFS(state.CWD)
		matches, err := doublestar.Glob(FS, pattern)
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

func (t *Task) run(ctx context.Context, log *log.Logger) hcl.Diagnostics {
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
	command := exec.CommandContext(ctx, terminal, "-c", script)
	command.Env = config.EnvSlice(t.Env)
	// todo: should this be configurable?
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	start := time.Now()
	err := command.Run()
	end := time.Now()
	// command.ProcessState.UserTime().String() provides inconsistent results
	// if the process is just iddling
	log.Println(`done in ` + end.Sub(start).String())
	// store results
	t.ExitCode = values.EventualInt64{
		Int64: int64(command.ProcessState.ExitCode()),
		Valid: true,
	}

	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		// just in case the program didn't output anything to std err
		detail = strings.TrimSpace(stdout.String())
	}

	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`"%s" task failed with "%s"`, AddressToString(t), command.ProcessState.String()),
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
			Summary:  fmt.Sprintf(`"%s" didn't create the expected file "%s"`, AddressToString(t), t.Creates),
			Subject:  &t.metadata.Creates,
			Context:  &t.metadata.Block,
		}}
	}

	return nil
}

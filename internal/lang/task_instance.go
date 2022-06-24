package lang

import (
	"fmt"
	"hash/crc64"
	"os"
	"path/filepath"
	"strconv"

	"bake/internal/concurrent"
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"bake/internal/lang/values"
	"bake/internal/paths"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type TaskInstance struct {
	Description string            `hcl:"description,optional"`
	Command     string            `hcl:"command,optional"`
	Creates     string            `hcl:"creates,optional"`
	Sources     []string          `hcl:"sources,optional"`
	Env         map[string]string `hcl:"env,optional"`
	Remain      hcl.Body          `hcl:",remain"`
	exitCode    values.EventualInt64
	path        cty.Path
	metadata    taskMetadata
}

func newTaskInstance(path cty.Path, metadata taskMetadata, body hcl.Body, ctx *hcl.EvalContext) (*TaskInstance, hcl.Diagnostics) {
	task := &TaskInstance{path: path, metadata: metadata}
	diags := gohcl.DecodeBody(body, ctx, task)
	if diags.HasErrors() {
		return nil, diags
	}

	diags = schema.ValidateAttributes(task.Remain)
	if diags.HasErrors() {
		return nil, diags
	}

	// make sure that irrelevant changes dont taint the state (example from ./dir/file to dir/file)
	if task.Creates != "" {
		task.Creates = filepath.Clean(task.Creates)
	}

	// overwrite default env with custom values
	task.Env = concurrent.Merge(config.Env(), task.Env)
	return task, nil
}

func (t TaskInstance) CTY() cty.Value {
	return values.StructToCty(t)
}

func (t TaskInstance) Hash() config.Hash {
	// somehow iterating over the map creates undeterministic results
	env := crc64.Checksum([]byte(fmt.Sprintf("%#v", t.Env)), crc64.MakeTable(crc64.ISO))
	command := crc64.Checksum([]byte(fmt.Sprintf("%#v", []byte(t.Command))), crc64.MakeTable(crc64.ISO))

	return config.Hash{
		Path:    paths.String(t.path),
		Creates: t.Creates,
		Command: strconv.FormatUint(command, 16),
		Env:     strconv.FormatUint(env, 16),
		Dirty:   !t.exitCode.Valid || t.exitCode.Int64 != 0,
	}
}

func (t *TaskInstance) Apply(state *config.State) hcl.Diagnostics {
	// don't apply twice in case more than 1 task depends on this
	if t.exitCode.Valid || t.Command == "" {
		return nil
	}

	log := NewLogger(t.path)
	if state.Flags.Prune {
		shouldRun, description, diags := t.dryPrune(state)
		if diags.HasErrors() {
			return diags
		}

		log.Println(description)
		if state.Flags.Dry {
			return nil
		}

		if !shouldRun && !state.Flags.Force {
			return nil
		}

		return t.prune(log)
	}

	// run by default
	shouldRun, description, diags := t.dryRun(state)
	if diags.HasErrors() {
		return diags
	}

	log.Println(description)
	if state.Flags.Dry {
		return nil
	}

	if !shouldRun && !state.Flags.Force {
		return nil
	}

	diags = t.run(state.Context, log)
	if diags.HasErrors() {
		return diags
	}

	// do we need to prune old stuff?
	oldHash, ok := state.Lock.Get(t.path)
	if !ok {
		return nil
	}

	if t.Creates == oldHash.Creates {
		return nil
	}

	err := os.RemoveAll(oldHash.Creates)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`error pruning %s task's old "creates": %s`, paths.String(t.path), oldHash.Creates),
			Detail:   err.Error(),
			Subject:  &t.metadata.Creates,
			Context:  &t.metadata.Block,
		}}
	}

	return nil
}

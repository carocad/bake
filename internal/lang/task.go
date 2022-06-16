package lang

import (
	"fmt"
	"hash/crc64"
	"os"
	"path/filepath"
	"strconv"

	"bake/internal/lang/config"
	"bake/internal/lang/meta"
	"bake/internal/lang/schema"
	"bake/internal/lang/values"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type Task struct {
	Name        string
	Description string            `hcl:"description,optional"`
	Command     string            `hcl:"command,optional"`
	Creates     string            `hcl:"creates,optional"`
	Sources     []string          `hcl:"sources,optional"`
	Env         map[string]string `hcl:"env,optional"`
	Remain      hcl.Body          `hcl:",remain"`
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
	t := TaskMetadata{Block: raw.Block.DefRange}
	diags := meta.DecodeRange(raw.Block.Body, ctx, &t)
	if diags.HasErrors() {
		return nil, diags
	}

	task := &Task{Name: raw.GetName(), metadata: t}
	diags = gohcl.DecodeBody(raw.Block.Body, ctx, task)
	if diags.HasErrors() {
		return nil, diags
	}

	diags = checkDependsOn(task.Remain)
	if diags.HasErrors() {
		return nil, diags
	}

	// make sure that irrelevant changes dont taint the state (example from ./dir/file to dir/file)
	if task.Creates != "" {
		task.Creates = filepath.Clean(task.Creates)
	}

	return task, nil
}

func (t Task) GetName() string {
	return t.Name
}

func (t Task) GetPath() cty.Path {
	// todo: change this to deal with for_each cases
	return cty.GetAttrPath(t.GetName())
}

func (t Task) GetFilename() string {
	return t.metadata.Block.Filename
}

func (t Task) CTY() cty.Value {
	return values.StructToCty(t)
}

func (t Task) Hash() *config.Hash {
	hasher := crc64.New(crc64.MakeTable(crc64.ISO))
	// somehow using a map[string]string returns non-deterministic results
	for _, v := range config.AppendEnv(t.Env) {
		hasher.Write([]byte(v))
	}
	env := hasher.Sum64()

	hasher.Reset()

	hasher.Write([]byte(t.Command))
	command := hasher.Sum64()

	return &config.Hash{
		Creates: t.Creates,
		Path:    t.GetPath(),
		Command: strconv.FormatUint(command, 16),
		Env:     strconv.FormatUint(env, 16),
		Dirty:   !t.ExitCode.Valid || t.ExitCode.Int64 != 0,
	}
}

func (t *Task) Apply(state config.State) hcl.Diagnostics {
	// don't apply twice in case more than 1 task depends on this
	if t.ExitCode.Valid || t.Command == "" {
		return nil
	}

	log := NewLogger(t)
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

	diags = t.run(log)
	if diags.HasErrors() {
		return diags
	}

	// do we need to prune old stuff?
	oldHash, ok := state.Lock.Tasks[AddressToString(t)]
	if !ok {
		return nil
	}

	hash := t.Hash()
	if hash.Creates == oldHash.Creates {
		return nil
	}

	err := os.RemoveAll(oldHash.Creates)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`error pruning %s task's old "creates": %s`, AddressToString(t), oldHash.Creates),
			Detail:   err.Error(),
			Subject:  &t.metadata.Creates,
			Context:  &t.metadata.Block,
		}}
	}

	return nil
}

func checkDependsOn(body hcl.Body) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return diags
	}

	for _, attr := range attrs {
		if attr.Name == schema.DependsOnAttr {
			_, diags := schema.TupleOfReferences(attrs[schema.DependsOnAttr])
			return diags
		}

		// only depends on is allowed
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Unsupported argument",
			Detail:   fmt.Sprintf(`An argument named "%s" is not expected here`, attr.Name),
			Subject:  attr.Expr.Range().Ptr(),
		}}
	}

	return nil
}

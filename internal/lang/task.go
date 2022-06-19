package lang

import (
	"context"
	"fmt"
	"hash/crc64"
	"os"
	"path/filepath"
	"strconv"

	"bake/internal/concurrent"
	"bake/internal/lang/config"
	"bake/internal/lang/meta"
	"bake/internal/lang/schema"
	"bake/internal/lang/values"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
	"github.com/zclconf/go-cty/cty/gocty"
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
	key         string // key is only valid for tasks with for_each attribute
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

func NewTasks(raw addressBlock, ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics) {
	forEachEntries, diags := getForEachEntries(raw, ctx)
	if diags.HasErrors() {
		return nil, diags
	}

	if forEachEntries == nil {
		task, diags := NewTask("", raw, ctx)
		if diags.HasErrors() {
			return nil, diags
		}

		return []Action{task}, nil
	}

	tasks := make([]Action, 0)
	for key, value := range forEachEntries {
		ctx := eachContext(key, value, ctx.NewChild())
		task, diags := NewTask(key, raw, ctx)
		if diags.HasErrors() {
			return nil, diags
		}

		tasks = append(tasks, task)
	}

	return tasks, nil
}

func getForEachEntries(raw addressBlock, ctx *hcl.EvalContext) (map[string]string, hcl.Diagnostics) {
	attributes, diags := raw.Block.Body.JustAttributes()
	if diags.HasErrors() {
		return nil, diags
	}

	for name, attr := range attributes {
		if name == schema.ForEachAttr {
			value, diags := attr.Expr.Value(ctx)
			if diags.HasErrors() {
				return nil, diags
			}

			forEachSet := make([]string, 0)
			err := gocty.FromCtyValue(value, &forEachSet)
			if err == nil {
				return concurrent.SetToMap(forEachSet), nil
			}

			diagnostic := hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary: fmt.Sprintf(
					`"for_each" field must be either a set or a map of strings but "%s" was provided`,
					value.Type().FriendlyNameForConstraint(),
				),
				Detail:      err.Error(),
				Subject:     attr.Expr.Range().Ptr(),
				Context:     &raw.Block.DefRange,
				Expression:  attr.Expr,
				EvalContext: ctx,
			}}
			forEachMap := make(map[string]string)
			// somehow FromCtyValue doesnt do this convertion itself :/
			if value.Type().IsObjectType() {
				v2, err := convert.Convert(value, cty.Map(cty.String))
				if err != nil {
					return nil, diagnostic
				}

				value = v2
			}

			err = gocty.FromCtyValue(value, &forEachMap)
			if err != nil {
				return nil, diagnostic
			}

			return forEachMap, nil
		}
	}

	return nil, nil
}

func eachContext(key, value string, context *hcl.EvalContext) *hcl.EvalContext {
	context.Variables = map[string]cty.Value{
		"each": cty.ObjectVal(map[string]cty.Value{
			"key":   cty.StringVal(key),
			"value": cty.StringVal(value),
		}),
	}

	return context
}

func NewTask(key string, raw addressBlock, ctx *hcl.EvalContext) (*Task, hcl.Diagnostics) {
	metadata := TaskMetadata{Block: raw.Block.DefRange}
	diags := meta.DecodeRange(raw.Block.Body, ctx, &metadata)
	if diags.HasErrors() {
		return nil, diags
	}

	task := &Task{Name: raw.GetName(), metadata: metadata, key: key}
	diags = gohcl.DecodeBody(raw.Block.Body, ctx, task)
	if diags.HasErrors() {
		return nil, diags
	}

	diags = verifyAttributes(task.Remain)
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

func (t Task) GetName() string {
	return t.Name
}

func (t Task) GetPath() cty.Path {
	path := cty.GetAttrPath(t.GetName())
	if t.key == "" {
		return path
	}

	return path.IndexString(t.key)
}

func (t Task) GetFilename() string {
	return t.metadata.Block.Filename
}

func (t Task) CTY() cty.Value {
	if t.key == "" {
		return values.StructToCty(t)
	}

	return cty.MapVal(map[string]cty.Value{
		t.key: values.StructToCty(t),
	})
}

func (t Task) Hash() *config.Hash {
	// somehow iterating over the map creates undeterministic results
	env := crc64.Checksum([]byte(fmt.Sprintf("%#v", t.Env)), crc64.MakeTable(crc64.ISO))
	command := crc64.Checksum([]byte(fmt.Sprintf("%#v", []byte(t.Command))), crc64.MakeTable(crc64.ISO))

	return &config.Hash{
		Creates: t.Creates,
		Path:    t.GetPath(),
		Command: strconv.FormatUint(command, 16),
		Env:     strconv.FormatUint(env, 16),
		Dirty:   !t.ExitCode.Valid || t.ExitCode.Int64 != 0,
	}
}

func (t *Task) Apply(ctx context.Context, state *config.State) hcl.Diagnostics {
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

	diags = t.run(ctx, log)
	if diags.HasErrors() {
		return diags
	}

	// do we need to prune old stuff?
	oldHash, ok := state.Lock.Tasks[AddressToString(t)]
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
			Summary:  fmt.Sprintf(`error pruning %s task's old "creates": %s`, AddressToString(t), oldHash.Creates),
			Detail:   err.Error(),
			Subject:  &t.metadata.Creates,
			Context:  &t.metadata.Block,
		}}
	}

	return nil
}

func verifyAttributes(body hcl.Body) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return diags
	}

	for _, attr := range attrs {
		if attr.Name == schema.DependsOnAttr {
			_, diags := schema.TupleOfReferences(attrs[schema.DependsOnAttr])
			return diags
		}

		if attr.Name == schema.ForEachAttr {
			continue
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

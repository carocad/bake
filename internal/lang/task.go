package lang

import (
	"context"
	"fmt"
	"hash/crc64"
	"os"
	"path/filepath"
	"strconv"
	"sync"

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
	"golang.org/x/exp/maps"
)

type TaskContainer struct {
	Name string
	// todo: how can I get this?
	// Description string `hcl:"description,optional"`

	metadata taskMetadata

	namedInstances   map[string]instance
	indexedInstances []instance
	singleInstance   instance
}

func newTaskContainer(raw addressBlock, ctx *hcl.EvalContext) (Action, hcl.Diagnostics) {
	forEachEntries, diags := getForEachEntries(raw, ctx)
	if diags.HasErrors() {
		return nil, diags
	}

	if forEachEntries == nil {
		task, diags := newTask(raw, ctx)
		if diags.HasErrors() {
			return nil, diags
		}

		return task, nil
	}

	instances := map[string]instance{}
	for key, value := range forEachEntries {
		ctx := eachContext(key, value, ctx.NewChild())
		task, diags := newTask(key, raw, ctx)
		if diags.HasErrors() {
			return nil, diags
		}

		instances[key] = task
	}

	return &TaskContainer{
		Name:           raw.GetName(),
		namedInstances: instances,
	}, nil
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

func (t TaskContainer) GetName() string {
	return t.Name
}

func (t TaskContainer) GetPath() cty.Path {
	return cty.GetAttrPath(t.GetName())
}

func (t TaskContainer) GetFilename() string {
	return t.metadata.Block.Filename
}

func (t TaskContainer) CTY() cty.Value {
	if len(t.namedInstances) > 0 {
		m := map[string]cty.Value{}
		for k, instance := range t.namedInstances {
			m[k] = instance.CTY()
		}

		return cty.MapVal(m)
	}

	if len(t.indexedInstances) > 0 {
		m := make([]cty.Value, len(t.namedInstances))
		for index, instance := range t.namedInstances {
			m[index] = instance.CTY()
		}

		return cty.ListVal(m)
	}

	return t.singleInstance.CTY()
}

func (t *TaskContainer) Apply(state *config.State) *sync.WaitGroup {
	if len(t.namedInstances) > 0 {
		return applyIndexed(maps.Values(t.namedInstances), state)
	}

	if len(t.indexedInstances) > 0 {
		return applyIndexed(t.indexedInstances, state)
	}

	return applySingle(t.singleInstance, state)

}

type Task struct {
	Command  string            `hcl:"command,optional"`
	Creates  string            `hcl:"creates,optional"`
	Sources  []string          `hcl:"sources,optional"`
	Env      map[string]string `hcl:"env,optional"`
	Remain   hcl.Body          `hcl:",remain"`
	ExitCode values.EventualInt64
}

type taskMetadata struct {
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

func newTask(key string, raw addressBlock, ctx *hcl.EvalContext) (*Task, hcl.Diagnostics) {
	metadata := taskMetadata{Block: raw.Block.DefRange}
	diags := meta.DecodeRange(raw.Block.Body, ctx, &metadata)
	if diags.HasErrors() {
		return nil, diags
	}

	task := &Task{metadata: metadata}
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

func (t Task) Hash() interface{} {
	// somehow iterating over the map creates undeterministic results
	env := crc64.Checksum([]byte(fmt.Sprintf("%#v", t.Env)), crc64.MakeTable(crc64.ISO))
	command := crc64.Checksum([]byte(fmt.Sprintf("%#v", []byte(t.Command))), crc64.MakeTable(crc64.ISO))

	return &config.Hash{
		Creates: t.Creates,
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

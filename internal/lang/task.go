package lang

import (
	"fmt"
	"sync"

	"bake/internal/concurrent"
	"bake/internal/lang/config"
	"bake/internal/lang/meta"
	"bake/internal/lang/schema"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
	"github.com/zclconf/go-cty/cty/gocty"
	"golang.org/x/exp/maps"
)

type Task struct {
	path             cty.Path
	metadata         taskMetadata
	namedInstances   map[string]*TaskInstance // for_each
	indexedInstances []*TaskInstance          // count?
	singleInstance   *TaskInstance            // plain task
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

func newTask(raw addressBlock, eval *hcl.EvalContext) (Action, hcl.Diagnostics) {
	path := raw.GetPath()
	metadata := taskMetadata{Block: raw.Block.DefRange}
	diags := meta.DecodeRange(raw.Block.Body, eval, &metadata)
	if diags.HasErrors() {
		return nil, diags
	}

	forEachEntries, diags := getForEachEntries(raw, eval)
	if diags.HasErrors() {
		return nil, diags
	}

	if forEachEntries == nil {
		task, diags := newTaskInstance(path, &metadata, raw.Block.Body, eval)
		if diags.HasErrors() {
			return nil, diags
		}

		return &Task{
			path:           path,
			singleInstance: task,
		}, nil
	}

	instances := map[string]*TaskInstance{}
	for key, value := range forEachEntries {
		ctx := eachContext(key, value, eval.NewChild())
		task, diags := newTaskInstance(path.IndexString(key), &metadata, raw.Block.Body, ctx)
		if diags.HasErrors() {
			return nil, diags
		}

		instances[key] = task
	}

	return &Task{
		path:           path,
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

func (t Task) GetPath() cty.Path {
	return t.path
}

func (t Task) GetFilename() string {
	return t.metadata.Block.Filename
}

func (t Task) CTY() cty.Value {
	if len(t.namedInstances) > 0 {
		m := map[string]cty.Value{}
		for k, instance := range t.namedInstances {
			m[k] = instance.CTY()
		}

		return cty.MapVal(m)
	}

	if len(t.indexedInstances) > 0 {
		m := make([]cty.Value, len(t.indexedInstances))
		for index, instance := range t.indexedInstances {
			m[index] = instance.CTY()
		}

		return cty.ListVal(m)
	}

	return t.singleInstance.CTY()
}

func (t *Task) Apply(state *config.State) *sync.WaitGroup {
	if len(t.namedInstances) > 0 {
		return applyIndexed(maps.Values(t.namedInstances), state)
	}

	if len(t.indexedInstances) > 0 {
		return applyIndexed(t.indexedInstances, state)
	}

	return applySingle(t.singleInstance, state)
}

func (t *Task) Hash() []config.Hash {
	result := make([]config.Hash, 0)
	if len(t.namedInstances) > 0 {
		for _, ri := range t.namedInstances {
			hash := ri.Hash()
			result = append(result, hash)
		}

		return result
	}

	if len(t.indexedInstances) > 0 {
		for _, ri := range t.indexedInstances {
			hash := ri.Hash()
			result = append(result, hash)
		}

		return result
	}

	hash := t.singleInstance.Hash()
	result = append(result, hash)
	return result
}

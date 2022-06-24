package lang

import (
	"sync"

	"bake/internal/lang/config"
	"bake/internal/lang/meta"
	"bake/internal/lang/schema"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
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
	DependsOn hcl.Range
}

func newTask(raw addressBlock, eval *hcl.EvalContext) (Action, hcl.Diagnostics) {
	path := raw.GetPath()
	metadata := taskMetadata{Block: raw.Block.DefRange}
	diags := meta.DecodeRange(raw.Block.Body, eval, &metadata)
	if diags.HasErrors() {
		return nil, diags
	}

	forEachEntries, diags := schema.ForEachEntries(raw.Block, eval)
	if diags.HasErrors() {
		return nil, diags
	}

	if len(forEachEntries) == 0 {
		task, diags := newTaskInstance(path, metadata, raw.Block.Body, eval)
		if diags.HasErrors() {
			return nil, diags
		}

		return &Task{
			path:           path,
			metadata:       metadata,
			singleInstance: task,
		}, nil
	}

	instances := map[string]*TaskInstance{}
	for key, value := range forEachEntries {
		ctx := eachContext(key, value, eval.NewChild())
		task, diags := newTaskInstance(path.IndexString(key), metadata, raw.Block.Body, ctx)
		if diags.HasErrors() {
			return nil, diags
		}

		instances[key] = task
	}

	return &Task{
		path:           path,
		metadata:       metadata,
		namedInstances: instances,
	}, nil
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

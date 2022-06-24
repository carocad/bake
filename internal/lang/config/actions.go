package config

import (
	"bake/internal/concurrent"
	"bake/internal/lang/schema"
	"bake/internal/paths"

	"github.com/zclconf/go-cty/cty"
)

type Actions []Action

func (actions Actions) EvalContext() map[string]cty.Value {
	variables := map[string]cty.Value{}
	data := map[string]cty.Value{}
	local := map[string]cty.Value{}
	task := map[string]cty.Value{}
	for _, act := range actions {
		path := act.GetPath()
		value := act.CTY()
		switch {
		case path.HasPrefix(schema.DataPrefix):
			name := paths.StepString(path[1])
			data[name] = value
		case path.HasPrefix(schema.LocalPrefix):
			name := paths.StepString(path[1])
			local[name] = value
		default:
			name := paths.StepString(path[0])
			task[name] = value
		}
	}

	variables[schema.DataLabel] = cty.ObjectVal(data)
	variables[schema.LocalScope] = cty.ObjectVal(local)
	variables[schema.TaskLabel] = cty.ObjectVal(task)
	// allow tasks to be referred without a prefix
	concurrent.Merge(variables, task)

	return variables
}

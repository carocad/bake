package config

import (
	"bake/internal/concurrent"
	"bake/internal/lang/schema"
	"bake/internal/paths"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function/stdlib"
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
		var err error
		switch {
		case path.HasPrefix(schema.DataPrefix):
			name := paths.StepString(path[1])
			data[name] = value
		case path.HasPrefix(schema.LocalPrefix):
			name := paths.StepString(path[1])
			local[name] = value
		default:
			name := paths.StepString(path[0])
			val, ok := task[name] // if the key already exist it is a map
			if ok {
				value, err = stdlib.Merge(value, val)
				if err != nil {
					panic(err.Error())
				}
			}

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

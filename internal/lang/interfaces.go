package lang

import (
	"bake/internal/concurrent"
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"bake/internal/lang/values"
	"bake/internal/paths"
	"log"
	"os"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mitchellh/colorstring"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

type Address interface {
	GetName() string
	GetPath() cty.Path
	GetFilename() string
}

type Action interface {
	Address
	values.Cty
	RuntimeAction
	config.Hasher
}

type RuntimeAction interface {
	Apply(*config.State) *sync.WaitGroup
}

type RuntimeInstance interface {
	Apply(*config.State) hcl.Diagnostics
}

type RawAddress interface {
	Address
	Dependencies() ([]hcl.Traversal, hcl.Diagnostics)
	Decode(ctx *hcl.EvalContext) (Action, hcl.Diagnostics)
}

func AddressToString[T Address](addr T) string {
	return paths.String(addr.GetPath())
}

type CliCommand struct{ Name, Description string }

func GetPublicTasks(addrs []RawAddress) []CliCommand {
	commands := make([]CliCommand, 0)
	for _, addr := range addrs {
		if schema.IsKnownPrefix(addr.GetPath()) {
			continue
		}

		// can only be task block
		block, ok := addr.(addressBlock)
		if !ok {
			continue
		}

		attrs, diags := block.Block.Body.JustAttributes()
		if diags.HasErrors() {
			continue
		}

		attr, ok := attrs[schema.DescriptionAttr]
		if !ok {
			continue
		}

		var description string
		diags = gohcl.DecodeExpression(attr.Expr, nil, &description)
		if diags.HasErrors() {
			continue
		}

		commands = append(commands, CliCommand{paths.String(addr.GetPath()), description})
	}

	return commands
}

func NewLogger(path cty.Path) *log.Logger {
	prefix := colorstring.Color("[bold]" + paths.String(path))
	// todo: change stdout according to state
	return log.New(os.Stdout, prefix+": ", 0)
}

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

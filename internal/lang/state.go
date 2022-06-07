package lang

import (
	"bake/internal/concurrent"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/mitchellh/colorstring"
	"github.com/zclconf/go-cty/cty"
)

type State struct {
	CWD string
	// Context     context.Context TODO
	Env         map[string]string
	Args        []string
	Dry         bool
	Prune       bool
	Force       bool
	Parallelism uint8
	Task        string
}

const DefaultParallelism = 4

func NewState(cwd string, task string) *State {
	// organize out env vars
	env := map[string]string{}
	for _, keyVal := range os.Environ() {
		parts := strings.SplitN(keyVal, "=", 2)
		key, val := parts[0], parts[1]
		env[key] = val
	}

	return &State{
		CWD:         cwd,
		Env:         env,
		Args:        os.Args,
		Parallelism: DefaultParallelism,
		Task:        task,
	}
}

func (state State) Context(addr RawAddress, actions []Action) *hcl.EvalContext {
	variables := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(state.CWD),
			"module":  cty.StringVal(filepath.Join(state.CWD, filepath.Dir(addr.GetFilename()))),
			"current": cty.StringVal(filepath.Join(state.CWD, addr.GetFilename())),
		}),
	}

	data := map[string]cty.Value{}
	local := map[string]cty.Value{}
	task := map[string]cty.Value{}
	for _, act := range actions {
		name := act.GetName()
		path := act.GetPath()
		value := act.CTY()
		switch {
		case path.HasPrefix(DataPrefix):
			data[name] = value
		case path.HasPrefix(LocalPrefix):
			local[name] = value
		default:
			task[name] = value
		}
	}

	variables[DataLabel] = cty.ObjectVal(data)
	variables[LocalScope] = cty.ObjectVal(local)
	variables[PathScope] = cty.ObjectVal(task)
	// allow tasks to be referred without a prefix
	concurrent.Merge(variables, task)
	return &hcl.EvalContext{
		Variables: variables,
		Functions: Functions(),
	}
}

func (state State) NewLogger(addr Address) *log.Logger {
	prefix := colorstring.Color("[bold]" + PathString(addr.GetPath()))
	// todo: change stdout according to state
	return log.New(os.Stdout, prefix+": ", 0)
}

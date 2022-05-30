package lang

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type State struct {
	CWD string
	// Context     context.Context TODO
	Env         map[string]string
	Args        []string
	DryRun      bool
	Prune       bool
	Parallelism uint8
	Task        string
}

const DefaultParallelism = 4

func NewState(cwd string, task string) *State {
	// organize out env vars

	return &State{
		CWD:         cwd,
		Env:         Env(),
		Args:        os.Args,
		Parallelism: DefaultParallelism,
		Task:        task,
	}
}

func Env() map[string]string {
	env := map[string]string{}
	environ := os.Environ()
	for _, keyVal := range environ {
		parts := strings.SplitN(keyVal, "=", 2)
		key, val := parts[0], parts[1]
		env[key] = val
	}

	return env
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
			// only targets for now !!
			variables[name] = value
		}
	}

	variables[DataLabel] = cty.ObjectVal(data)
	variables[LocalScope] = cty.ObjectVal(local)
	return &hcl.EvalContext{
		Variables: variables,
		Functions: Functions(),
	}
}

func (state State) NewLogger(addr Address) *log.Logger {
	prefix := PathString(addr.GetPath())
	// todo: change stdout according to state
	return log.New(os.Stdout, prefix+": ", 0)
}

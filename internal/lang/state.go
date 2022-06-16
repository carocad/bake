package lang

import (
	"bake/internal/concurrent"
	"fmt"
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
	Flags       StateFlags
	Parallelism uint8
	Lock        *Lock
}

type StateFlags struct {
	Dry   bool
	Prune bool
	Force bool
}

func NewStateFlags(dry, prune, force bool) (StateFlags, error) {
	if dry && force {
		return StateFlags{}, fmt.Errorf(`"dry" and "force" are contradictory flags`)
	}

	return StateFlags{
		Dry:   dry,
		Prune: prune,
		Force: force,
	}, nil
}

const DefaultParallelism = 4

func NewState() (*State, error) {
	// where are we?
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// organize out env vars
	env := map[string]string{}
	for _, keyVal := range os.Environ() {
		parts := strings.SplitN(keyVal, "=", 2)
		key, val := parts[0], parts[1]
		env[key] = val
	}

	// fetch state from filesystem
	lock, err := lockFromFilesystem(cwd)
	if err != nil {
		return nil, err
	}

	return &State{
		CWD:         cwd,
		Env:         env,
		Args:        os.Args,
		Lock:        lock,
		Parallelism: DefaultParallelism,
	}, nil
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
	variables[TaskLabel] = cty.ObjectVal(task)
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

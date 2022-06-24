package config

import (
	"bake/internal/lang/schema"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/sync/errgroup"
)

type State struct {
	CWD     string
	Context context.Context
	args    []string
	Flags   StateFlags
	Lock    *Lock
	Group   *errgroup.Group
}

const DefaultParallelism = 4

func NewState(ctx context.Context) (*State, error) {
	bounded, ctx := errgroup.WithContext(ctx)
	// todo: read this from env vars or similar
	bounded.SetLimit(DefaultParallelism)
	// where are we?
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// fetch state from filesystem
	lock, err := lockFromFilesystem(cwd)
	if err != nil {
		return nil, err
	}

	return &State{
		CWD:     cwd,
		args:    os.Args,
		Lock:    lock,
		Context: ctx,
		Group:   bounded,
	}, nil
}

func (state State) EvalContext() *hcl.EvalContext {
	args := make([]cty.Value, len(state.args))
	for index, arg := range state.args {
		args[index] = cty.StringVal(arg)
	}

	env := map[string]cty.Value{}
	for key, val := range Env() {
		env[key] = cty.StringVal(val)
	}

	variables := map[string]cty.Value{
		"process": cty.ObjectVal(map[string]cty.Value{
			"args": cty.ListVal(args),
			"env":  cty.MapVal(env),
		}),
	}

	ctx := hcl.EvalContext{
		Variables: variables,
		Functions: schema.Functions(),
	}
	return ctx.NewChild()
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

func Env() map[string]string {
	// organize out env vars
	env := map[string]string{}
	for _, keyVal := range os.Environ() {
		parts := strings.SplitN(keyVal, "=", 2)
		key, val := parts[0], parts[1]
		env[key] = val
	}

	return env
}

func EnvSlice(input map[string]string) []string {
	env := make([]string, 0)
	for k, v := range input {
		env = append(env, k+"="+v)
	}

	return env
}

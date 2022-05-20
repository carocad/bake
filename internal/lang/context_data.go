package lang

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type ContextData struct {
	CWD  string
	Env  map[string]string
	Args []string
	// todo: try to generalize this
	DryRun      bool
	Parallelism uint8
}

const DefaultParallelism = 4

func NewContextData(cwd string, dryRun bool) ContextData {
	env := map[string]string{}
	environ := os.Environ()
	for _, keyVal := range environ {
		parts := strings.SplitN(keyVal, "=", 2)
		key, val := parts[0], parts[1]
		env[key] = val
	}

	return ContextData{
		CWD:         cwd,
		Env:         env,
		Args:        os.Args,
		DryRun:      dryRun,
		Parallelism: DefaultParallelism,
	}
}

func (ctx ContextData) Context(addr RawAddress, actions []Action) *hcl.EvalContext {
	variables := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(ctx.CWD),
			"module":  cty.StringVal(filepath.Join(ctx.CWD, filepath.Dir(addr.GetFilename()))),
			"current": cty.StringVal(filepath.Join(ctx.CWD, addr.GetFilename())),
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

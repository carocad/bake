package state

import (
	"bake/internal/lang"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Config struct {
	CWD string
	// Context     context.Context TODO
	Env         map[string]string
	Args        []string
	DryRun      bool // todo: try to generalize this
	Parallelism uint8
	Task        string
}

const DefaultParallelism = 4

func NewConfig(cwd string) *Config {
	// organize out env vars

	return &Config{
		CWD:         cwd,
		Env:         Env(),
		Args:        os.Args,
		Parallelism: DefaultParallelism,
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

func (ctx Config) Context(addr lang.RawAddress, actions []lang.Action) *hcl.EvalContext {
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
		case path.HasPrefix(lang.DataPrefix):
			data[name] = value
		case path.HasPrefix(lang.LocalPrefix):
			local[name] = value
		default:
			// only targets for now !!
			variables[name] = value
		}
	}

	variables[lang.DataLabel] = cty.ObjectVal(data)
	variables[lang.LocalScope] = cty.ObjectVal(local)
	return &hcl.EvalContext{
		Variables: variables,
		Functions: lang.Functions(),
	}
}

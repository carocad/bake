package module

import (
	"bake/internal/lang"
	"bake/internal/module/contextualize"
	"bake/internal/module/worker"
	"bake/internal/topo"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Module struct {
	// name by which the module is known; by convention the root module
	// doesn't have a name as it is "global"
	name string
	cwd  string
}

func (module Module) Plan(target string, filePartials map[string][]lang.RawAddress) ([]lang.Action, hcl.Diagnostics) {
	for _, addresses := range filePartials {
		for _, act := range addresses {

			if lang.PathString(act.Path()) != target {
				continue
			}

			deps, diags := topo.Dependencies(act, filePartials, lang.GlobalPrefixes)
			if diags.HasErrors() {
				return nil, diags
			}

			semaphore := contextualize.NewSemaphore(module.cwd, filePartials)
			actions, diagnostics := worker.DO(semaphore, deps)
			if diagnostics.HasErrors() {
				return nil, diagnostics
			}

			return actions, nil
		}
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "couldn't find any target with name " + target,
	}}
}

func NewRootModule(cwd string) *Module {
	return NewModule("", cwd)
}

func NewModule(name, cwd string) *Module {
	return &Module{name: name, cwd: cwd}
}

func (module Module) Path() cty.Path {
	// root module
	if module.name == "" {
		return cty.Path{}
	}

	return cty.GetAttrPath("module").GetAttr(module.name)
}

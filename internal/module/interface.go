package module

import (
	"context"
	"sync"

	"bake/internal/lang"
	"bake/internal/routine"
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

			actGroup := make([][]lang.Action, len(deps))
			coordinator := routine.WithContext(context.TODO())
			mutex := sync.RWMutex{}
			for index, dep := range deps {
				index, dep := index, dep
				requires, diags := topo.Dependencies(dep, filePartials, lang.GlobalPrefixes)
				if diags.HasErrors() {
					return nil, diags
				}

				ids := lang.DependencyIds(requires[:len(requires)-1])
				coordinator.Do(lang.PathString(dep.Path()), ids, func() error {
					eval := module.Context(dep, filePartials, actGroup)
					actions, diags := dep.Decode(eval)
					if diags.HasErrors() {
						return diags
					}

					if !dep.Path().HasPrefix(lang.DataPrefix) {
						mutex.Lock()
						defer mutex.Unlock()
						actGroup[index] = actions
						return nil
					}

					// we need to refresh before the next actions are loaded since
					// they depend on the data values
					for _, action := range actions {
						diags := action.Apply()
						if diags.HasErrors() {
							return diags
						}
					}

					mutex.Lock()
					defer mutex.Unlock()
					actGroup[index] = actions
					return nil
				})
			}
			err := coordinator.Wait()
			if diags, ok := err.(hcl.Diagnostics); ok {
				return nil, diags
			}

			if err != nil {
				return nil, hcl.Diagnostics{{
					Severity: hcl.DiagError,
					Summary:  err.Error(),
				}}
			}

			result := make([]lang.Action, 0)
			for _, actions := range actGroup {
				result = append(result, actions...)
			}
			return result, nil
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

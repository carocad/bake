package module

import (
	"bake/internal/lang"
	"bake/internal/module/action"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

const dataName = "data"

var (
	phonyData   = cty.GetAttrPath(lang.PhonyLabel).GetAttr(dataName)
	phonyPrefix = cty.GetAttrPath(lang.PhonyLabel)
	localPrefix = cty.GetAttrPath(lang.LocalScope)
	pathPrefix  = cty.GetAttrPath(lang.PathScope)
)

func (module Module) Plan(target string, fileAddrs FileMapping) ([]action.Action, hcl.Diagnostics) {
	allActions := make([]action.Action, 0)
	sorter := topologicalSort{
		fileAddrs: fileAddrs,
		markers:   map[string]*addressMark{},
		global:    cty.NewPathSet(pathPrefix),
	}
	for filename, addresses := range fileAddrs {
		for _, act := range addresses {

			if act.Path().HasPrefix(localPrefix) {
				continue
			}

			if act.GetName() != target {
				continue
			}

			deps, diags := sorter.dependencies(act)
			if diags.HasErrors() {
				return nil, diags
			}

			for _, dep := range deps {
				context, diags := module.currentContext(filename, fileAddrs)
				if diags.HasErrors() {
					return nil, diags
				}

				actions, diags := dep.Plan(context)
				if diags.HasErrors() {
					return nil, diags
				}

				allActions = append(allActions, actions...)

				// preload phony.data and locals ...
				if !dep.Path().Equals(phonyData) {
					continue
				}

				// refactor this
				for _, act := range actions {
					diags := act.Apply()
					if diags.HasErrors() {
						return nil, diags
					}
				}
			}
		}

		return allActions, nil
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "couldn't find any target with name " + target,
	}}
}

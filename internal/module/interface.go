package module

import (
	"bake/internal/lang"
	"bake/internal/module/action"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

const dataName = "data"

var phonyData = cty.GetAttrPath(lang.PhonyLabel).GetAttr(dataName)

func (module Module) Plan(target string) ([]action.Action, hcl.Diagnostics) {
	allActions := make([]action.Action, 0)
	for _, act := range module.addresses {
		if act.GetName() != target {
			continue
		}

		deps, diags := module.dependencies(act)
		if diags.HasErrors() {
			return nil, diags
		}

		for _, dep := range deps {
			context, diags := module.currentContext()
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

		return allActions, nil
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "couldn't find any target with name " + target,
	}}
}

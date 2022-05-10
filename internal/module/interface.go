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
	for _, act := range module.actions {
		if act.GetName() != target {
			continue
		}

		deps, diags := module.dependencies(act)
		if diags.HasErrors() {
			return nil, diags
		}

		// pre-load phony.data and locals ...
		for _, dep := range deps {
			if !dep.Path().Equals(phonyData) {
				continue
			}

			diags = dep.Run()
			if diags.HasErrors() {
				return nil, diags
			}
		}

		return deps, nil
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "couldn't find any target with name " + target,
	}}
}

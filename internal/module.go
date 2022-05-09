package internal

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Module struct {
	Actions []Action
	name    string
}

func (module Module) Path() cty.Path {
	// root module
	if module.name == "" {
		return cty.Path{}
	}

	return cty.GetAttrPath("module").GetAttr(module.name)
}

type mark int

const (
	unmarked = iota
	temporary
	permanent
)

type actionMark struct {
	Action
	mark
}

// Dependencies according to
// https://www.wikiwand.com/en/Topological_sorting#/Depth-first_search
// NOTE: the task itself is the last element of the dependency list
func (module Module) Dependencies(task Action) ([]Action, hcl.Diagnostics) {
	markers := make(map[string]*actionMark)
	markers[task.GetName()] = &actionMark{task, unmarked}
	order, diags := module.visit(task.GetName(), markers)
	if diags.HasErrors() {
		return nil, diags
	}

	return order, nil
}

const cyclicalDependency = "cyclical dependency detected"

func (module Module) visit(current string, markers map[string]*actionMark) ([]Action, hcl.Diagnostics) {
	id := markers[current]
	if id.mark == permanent {
		return nil, nil
	}

	if id.mark == temporary {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  cyclicalDependency,
			Detail:   current,
		}}
	}

	id.mark = temporary
	order := make([]Action, 0)
	for _, dep := range id.Dependencies() {
		innerID, diags := module.GetByID(dep)
		if diags.HasErrors() {
			return nil, diags
		}

		// make sure we initialize the marker
		if _, found := markers[innerID.GetName()]; !found {
			markers[innerID.GetName()] = &actionMark{innerID, unmarked}
		}

		inner, diags := module.visit(innerID.GetName(), markers)
		if diags.HasErrors() {
			for _, diag := range diags {
				if diag.Summary == cyclicalDependency {
					diag.Detail = fmt.Sprintf("%s -> %s", current, diag.Detail)
				}
			}
			return nil, diags
		}

		order = append(order, inner...)
	}
	id.mark = permanent
	order = append(order, id.Action)
	return order, nil
}

func (module Module) GetByID(traversal hcl.Traversal) (Action, hcl.Diagnostics) {
	path := ToPath(traversal)
	for _, action := range module.Actions {
		if action.Path().Equals(path) {
			return action, nil
		}
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "unknown reference",
		Subject:  traversal.SourceRange().Ptr(),
	}}
}

func ToPath(traversal hcl.Traversal) cty.Path {
	path := cty.Path{}
	for _, step := range traversal {
		switch traverse := step.(type) {
		case hcl.TraverseRoot:
			path = path.GetAttr(traverse.Name)
		case hcl.TraverseAttr:
			path = path.GetAttr(traverse.Name)
		default:
			// fail fast -> implement more cases as bake evolves
			panic(fmt.Sprintf("unknown traversal step '%s'", traverse))
		}
	}

	return path
}

// TargetScope is not necessary since those are "attached" directly to the "module"
const (
	PhonyScope  = "phony"
	LocalScope  = "local"
	ModuleScope = "module"
)

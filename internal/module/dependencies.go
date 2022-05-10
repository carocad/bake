package module

import (
	"fmt"

	"bake/internal/lang"
	"bake/internal/module/action"
	"github.com/hashicorp/hcl/v2"
)

type mark int

const (
	unmarked = iota
	temporary
	permanent
)

type actionMark struct {
	action.Action
	mark
}

// dependencies according to
// https://www.wikiwand.com/en/Topological_sorting#/Depth-first_search
// NOTE: the task itself is the last element of the dependency list
func (module Module) dependencies(task action.Action) ([]action.Action, hcl.Diagnostics) {
	markers := make(map[string]*actionMark)
	path := lang.PathString(task.Path())
	markers[path] = &actionMark{task, unmarked}
	order, diags := module.visit(path, markers)
	if diags.HasErrors() {
		return nil, diags
	}

	return order, nil
}

const cyclicalDependency = "cyclical dependency detected"

func (module Module) visit(current string, markers map[string]*actionMark) ([]action.Action, hcl.Diagnostics) {
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
	order := make([]action.Action, 0)
	for _, dep := range id.Dependencies() {
		innerID, diags := module.getByPrefix(dep)
		if diags.HasErrors() {
			return nil, diags
		}

		// make sure we initialize the marker
		path := lang.PathString(innerID.Path())
		if _, found := markers[path]; !found {
			markers[path] = &actionMark{innerID, unmarked}
		}

		inner, diags := module.visit(path, markers)
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

func (module Module) getByPrefix(traversal hcl.Traversal) (action.Action, hcl.Diagnostics) {
	path := lang.ToPath(traversal)
	for _, act := range module.actions {
		if path.HasPrefix(act.Path()) {
			return act, nil
		}
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "unknown reference",
		Subject:  traversal.SourceRange().Ptr(),
	}}
}

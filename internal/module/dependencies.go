package module

import (
	"fmt"

	"bake/internal/lang"
	"bake/internal/module/action"
	"github.com/agext/levenshtein"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// dependencies according to
// https://www.wikiwand.com/en/Topological_sorting#/Depth-first_search
// NOTE: the task itself is the last element of the dependency list
type topologicalSort struct {
	fileAddrs FileMapping
	markers   map[string]*addressMark
	global    cty.PathSet
}

type mark int

const (
	unmarked = iota
	temporary
	permanent
)

type addressMark struct {
	action.Address
	mark
}

func (topo topologicalSort) dependencies(addr action.Address) ([]action.Address, hcl.Diagnostics) {
	path := lang.PathString(addr.Path())
	topo.markers[path] = &addressMark{addr, unmarked}
	order, diags := topo.visit(path)
	if diags.HasErrors() {
		return nil, diags
	}

	return order, nil
}

const cyclicalDependency = "cyclical dependency detected"

func (topo topologicalSort) visit(current string) ([]action.Address, hcl.Diagnostics) {
	id := topo.markers[current]
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
	order := make([]action.Address, 0)
	for _, dep := range id.Dependencies() {
		if topo.ignoreRef(dep) {
			continue
		}

		innerID, diags := topo.getByPrefix(dep)
		if diags.HasErrors() {
			return nil, diags
		}

		// make sure we initialize the marker
		path := lang.PathString(innerID.Path())
		if _, found := topo.markers[path]; !found {
			topo.markers[path] = &addressMark{innerID, unmarked}
		}

		inner, diags := topo.visit(path)
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
	order = append(order, id.Address)
	return order, nil
}

func (topo topologicalSort) getByPrefix(traversal hcl.Traversal) (action.Address, hcl.Diagnostics) {
	path := lang.ToPath(traversal)
	for _, addresses := range topo.fileAddrs {
		for _, act := range addresses {
			if path.HasPrefix(act.Path()) {
				return act, nil
			}
		}
	}

	suggestion := topo.suggest(path)
	summary := "unknown reference"
	if suggestion != "" {
		summary += fmt.Sprintf(`, did you mean "%s"?`, suggestion)
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  summary,
		Subject:  traversal.SourceRange().Ptr(),
	}}
}

func (topo topologicalSort) ignoreRef(traversal hcl.Traversal) bool {
	traversalPath := lang.ToPath(traversal)
	for _, path := range topo.global.List() {
		if traversalPath.HasPrefix(path) {
			return true
		}
	}

	return false
}

func (topo topologicalSort) suggest(search cty.Path) string {
	searchText := lang.PathString(search)
	suggestion := ""
	bestDistance := len(searchText)
	for _, addresses := range topo.fileAddrs {
		for _, addr := range addresses {
			typo := lang.PathString(addr.Path())
			dist := levenshtein.Distance(searchText, typo, nil)
			if dist < bestDistance {
				suggestion = typo
				bestDistance = dist
			}
		}
	}

	return suggestion
}

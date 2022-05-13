package topo

import (
	"fmt"

	"bake/internal/lang"
	"bake/internal/module/action"
	"github.com/agext/levenshtein"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type depthFirst struct {
	fileAddrs map[string][]action.Address
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

// Dependencies sorting according to
// https://www.wikiwand.com/en/Topological_sorting#/Depth-first_search
// NOTE: the task itself is the last element of the dependency list
func Dependencies(addr action.Address, fileAddrs map[string][]action.Address, globals cty.PathSet) ([]action.Address, hcl.Diagnostics) {
	path := lang.PathString(addr.Path())
	sorter := depthFirst{
		fileAddrs: fileAddrs,
		markers: map[string]*addressMark{
			path: {addr, unmarked},
		},
		global: globals,
	}

	order, diags := sorter.visit(path)
	if diags.HasErrors() {
		return nil, diags
	}

	return order, nil
}

const cyclicalDependency = "cyclical dependency detected"

func (sorter depthFirst) visit(current string) ([]action.Address, hcl.Diagnostics) {
	id := sorter.markers[current]
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
		if sorter.ignoreRef(dep) {
			continue
		}

		innerID, diags := sorter.getByPrefix(dep)
		if diags.HasErrors() {
			return nil, diags
		}

		// make sure we initialize the marker
		path := lang.PathString(innerID.Path())
		if _, found := sorter.markers[path]; !found {
			sorter.markers[path] = &addressMark{innerID, unmarked}
		}

		inner, diags := sorter.visit(path)
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

func (sorter depthFirst) getByPrefix(traversal hcl.Traversal) (action.Address, hcl.Diagnostics) {
	path := lang.ToPath(traversal)
	for _, addresses := range sorter.fileAddrs {
		for _, act := range addresses {
			if path.HasPrefix(act.Path()) {
				return act, nil
			}
		}
	}

	suggestion := sorter.suggest(path)
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

func (sorter depthFirst) ignoreRef(traversal hcl.Traversal) bool {
	traversalPath := lang.ToPath(traversal)
	for _, path := range sorter.global.List() {
		if traversalPath.HasPrefix(path) {
			return true
		}
	}

	return false
}

func (sorter depthFirst) suggest(search cty.Path) string {
	searchText := lang.PathString(search)
	suggestion := ""
	bestDistance := len(searchText)
	for _, addresses := range sorter.fileAddrs {
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

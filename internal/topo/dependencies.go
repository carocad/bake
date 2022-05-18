package topo

import (
	"fmt"

	"bake/internal/functional"
	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type depthFirst struct {
	addresses []lang.RawAddress
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
	lang.RawAddress
	mark
}

// AllDependencies returns a map of address string to raw addresses
func AllDependencies(task lang.RawAddress, addresses []lang.RawAddress) (map[string][]lang.RawAddress, hcl.Diagnostics) {
	deps, diags := Dependencies(task, addresses)
	if diags.HasErrors() {
		return nil, diags
	}

	result := map[string][]lang.RawAddress{}
	for _, dep := range deps {
		inner, diags := Dependencies(dep, addresses)
		if diags.HasErrors() {
			return nil, diags
		}

		result[lang.RawAddressToString(dep)] = inner
	}

	return result, nil
}

// Dependencies sorting according to
// https://www.wikiwand.com/en/Topological_sorting#/Depth-first_search
// NOTE: the task itself is the last element of the dependency list
func Dependencies(addr lang.RawAddress, addresses []lang.RawAddress) ([]lang.RawAddress, hcl.Diagnostics) {
	path := lang.PathString(addr.GetPath())
	sorter := depthFirst{
		addresses: addresses,
		markers: map[string]*addressMark{
			path: {addr, unmarked},
		},
		global: lang.GlobalPrefixes,
	}

	order, diags := sorter.visit(path)
	if diags.HasErrors() {
		return nil, diags
	}

	return order, nil
}

const cyclicalDependency = "cyclical dependency detected"

func (sorter depthFirst) visit(current string) ([]lang.RawAddress, hcl.Diagnostics) {
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
	order := make([]lang.RawAddress, 0)
	dependencies, diagnostics := id.Dependencies()
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	for _, dep := range dependencies {
		if sorter.ignoreRef(dep) {
			continue
		}

		innerID, diags := sorter.getByPrefix(dep)
		if diags.HasErrors() {
			return nil, diags
		}

		// make sure we initialize the marker
		path := lang.PathString(innerID.GetPath())
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
	order = append(order, id.RawAddress)
	return order, nil
}

func (sorter depthFirst) getByPrefix(traversal hcl.Traversal) (lang.RawAddress, hcl.Diagnostics) {
	path := lang.ToPath(traversal)
	for _, address := range sorter.addresses {
		if path.HasPrefix(address.GetPath()) {
			return address, nil
		}
	}

	options := functional.Map(sorter.addresses, lang.RawAddressToString)
	suggestion := functional.Suggest(lang.PathString(path), options)
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

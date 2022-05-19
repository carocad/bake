package topo

import (
	"fmt"

	"bake/internal/functional"
	"bake/internal/lang"

	"github.com/hashicorp/hcl/v2"
)

type marker int

const (
	unmarked = iota
	temporary
	permanent
)

// AllDependencies returns a map of address string to addresses
func AllDependencies[T lang.Address](task T, addresses []T) (map[string][]T, hcl.Diagnostics) {
	deps, diags := Dependencies(task, addresses)
	if diags.HasErrors() {
		return nil, diags
	}

	result := map[string][]T{}
	for _, dep := range deps {
		inner, diags := Dependencies(dep, addresses)
		if diags.HasErrors() {
			return nil, diags
		}

		result[lang.AddressToString(dep)] = inner
	}

	return result, nil
}

// Dependencies sorting according to
// https://www.wikiwand.com/en/Topological_sorting#/Depth-first_search
// NOTE: the task itself is the last element of the dependency list
func Dependencies[T lang.Address](addr T, addresses []T) ([]T, hcl.Diagnostics) {
	mapping := map[string]T{}
	for _, address := range addresses {
		mapping[lang.AddressToString(address)] = address
	}

	path := lang.PathString(addr.GetPath())
	markers := map[string]marker{
		path: unmarked,
	}

	order, diags := visit(path, markers, mapping)
	if diags.HasErrors() {
		return nil, diags
	}

	return order, nil
}

const cyclicalDependency = "cyclical dependency detected"

func visit[T lang.Address](current string, markers map[string]marker, addresses map[string]T) ([]T, hcl.Diagnostics) {
	mark := markers[current]
	if mark == permanent {
		return nil, nil
	}

	if mark == temporary {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  cyclicalDependency,
			Detail:   current,
		}}
	}

	markers[current] = temporary
	order := make([]T, 0)
	dependencies, diagnostics := addresses[current].Dependencies()
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	for _, dep := range dependencies {
		if ignoreRef(dep) {
			continue
		}

		innerID, diags := getByPrefix(dep, addresses)
		if diags.HasErrors() {
			return nil, diags
		}

		// make sure we initialize the marker
		path := lang.AddressToString(*innerID)
		inner, diags := visit(path, markers, addresses)
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
	markers[current] = permanent
	order = append(order, addresses[current])
	return order, nil
}

func getByPrefix[T lang.Address](traversal hcl.Traversal, addresses map[string]T) (*T, hcl.Diagnostics) {
	path := lang.ToPath(traversal)
	for _, address := range addresses {
		if path.HasPrefix(address.GetPath()) {
			return &address, nil
		}
	}

	options := functional.Map(functional.Values(addresses), lang.AddressToString[T])
	suggestion := functional.Suggest(lang.PathString(path), options)
	summary := "unknown reference"
	if suggestion != "" {
		summary += fmt.Sprintf(`. Did you mean "%s"?`, suggestion)
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  summary,
		Subject:  traversal.SourceRange().Ptr(),
	}}
}

func ignoreRef(traversal hcl.Traversal) bool {
	traversalPath := lang.ToPath(traversal)
	for _, path := range lang.GlobalPrefixes.List() {
		if traversalPath.HasPrefix(path) {
			return true
		}
	}

	return false
}

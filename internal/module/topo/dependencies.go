package topo

import (
	"fmt"

	"bake/internal/functional"
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"bake/internal/paths"

	"github.com/hashicorp/hcl/v2"
)

type marker int

const (
	unmarked = iota
	temporary
	permanent
)

// AllDependencies returns a map of address string to addresses
func AllDependencies(task config.RawAddress, addresses []config.RawAddress) (map[string][]config.RawAddress, hcl.Diagnostics) {
	deps, diags := Dependencies(task, addresses)
	if diags.HasErrors() {
		return nil, diags
	}

	result := map[string][]config.RawAddress{}
	for _, dep := range deps {
		inner, diags := Dependencies(dep, addresses)
		if diags.HasErrors() {
			return nil, diags
		}

		result[config.AddressToString(dep)] = inner
	}

	return result, nil
}

// Dependencies sorting according to
// https://www.wikiwand.com/en/Topological_sorting#/Depth-first_search
// NOTE: the task itself is the last element of the dependency list
func Dependencies(addr config.RawAddress, addresses []config.RawAddress) ([]config.RawAddress, hcl.Diagnostics) {
	mapping := map[string]config.RawAddress{}
	for _, address := range addresses {
		mapping[config.AddressToString(address)] = address
	}

	path := config.AddressToString(addr)
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

func visit(current string, markers map[string]marker, addresses map[string]config.RawAddress) ([]config.RawAddress, hcl.Diagnostics) {
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
	order := make([]config.RawAddress, 0)
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
		path := config.AddressToString(*innerID)
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

func getByPrefix[T config.Address](traversal hcl.Traversal, addresses map[string]T) (*T, hcl.Diagnostics) {
	path := paths.FromTraversal(traversal)
	for _, address := range addresses {
		if path.HasPrefix(address.GetPath()) {
			return &address, nil
		}
	}

	options := functional.Map(functional.Values(addresses), config.AddressToString[T])
	suggestion := functional.Suggest(paths.String(path), options)
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
	traversalPath := paths.FromTraversal(traversal)
	for _, path := range schema.IgnorePrefixes.List() {
		if traversalPath.HasPrefix(path) {
			return true
		}
	}

	return false
}

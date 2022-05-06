package internal

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

type mark int

const (
	unmarked = iota
	temporary
	permanent
)

type identifiableMark struct {
	Identifiable
	mark
}

// Dependencies according to
// https://www.wikiwand.com/en/Topological_sorting#/Depth-first_search
// NOTE: the task itself is the last element of the dependency list
func (recipe Recipe) Dependencies(task Identifiable) ([]Identifiable, hcl.Diagnostics) {
	markers := make(map[string]*identifiableMark)
	markers[task.GetName()] = &identifiableMark{task, unmarked}
	order, diags := recipe.visit(task.GetName(), markers)
	if diags.HasErrors() {
		return nil, diags
	}

	return order, nil
}

func (recipe Recipe) visit(current string, markers map[string]*identifiableMark) ([]Identifiable, hcl.Diagnostics) {
	id := markers[current]
	if id.mark == permanent {
		return nil, nil
	}

	if id.mark == temporary {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "cyclical dependency detected",
		}}
	}

	id.mark = temporary
	order := make([]Identifiable, 0)
	for _, dep := range id.Dependencies() {
		innerID, diags := recipe.GetByID(dep)
		if diags.HasErrors() {
			return nil, diags
		}

		// make sure we initialize the marker
		if _, found := markers[innerID.GetName()]; !found {
			markers[innerID.GetName()] = &identifiableMark{innerID, unmarked}
		}

		inner, diags := recipe.visit(innerID.GetName(), markers)
		if diags.HasErrors() {
			return nil, diags
		}

		order = append(order, inner...)
	}
	id.mark = permanent
	order = append(order, id.Identifiable)
	return order, nil
}

const detail = `A reference to a %s type must be followed by at least one attribute access, specifying its name.`

func (recipe Recipe) GetByID(traversal hcl.Traversal) (Identifiable, hcl.Diagnostics) {
	root := traversal.RootName()
	switch root {
	case "phony":
		if len(traversal) < 2 {
			return nil, hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "Invalid reference",
				Detail:   fmt.Sprintf(detail, "phony"),
				Subject:  traversal.SourceRange().Ptr(),
			}}
		}

		relative := traversal[1]
		switch tt := relative.(type) {
		case hcl.TraverseAttr:
			name := tt.Name
			for _, phony := range recipe.Phonies {
				if phony.Name == name {
					return Identifiable(phony), nil
				}
			}
		default:
			return nil, hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "invalid reference",
				Detail:   fmt.Sprintf(detail, "phony"),
				Subject:  relative.SourceRange().Ptr(),
			}}
		}
	default: // target
		for _, target := range recipe.Targets {
			if target.Name == root {
				return Identifiable(target), nil
			}
		}
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "invalid reference",
		Detail:   fmt.Sprintf(detail, "phony"),
		Subject:  traversal.SourceRange().Ptr(),
	}}
}

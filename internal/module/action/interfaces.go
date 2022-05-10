package action

import (
	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type EventualStatus int

const (
	Pending EventualStatus = iota
	Running
	Completed
)

type Action interface {
	GetName() string
	Dependencies() []hcl.Traversal
	Status() EventualStatus
	Run() hcl.Diagnostics

	// Settle forces evaluation of expressions that depend on other
	// actions
	// Settle() hcl.Diagnostics

	Addressable
}

type Addressable interface {
	Path() cty.Path
}

func dependsOn(attrs hcl.Attributes) ([]hcl.Traversal, hcl.Diagnostics) {
	diagnostics := make(hcl.Diagnostics, 0)
	for name, attr := range attrs {
		if name == lang.DependsOnAttr {
			variables, diags := lang.TupleOfReferences(attr)
			if diags.HasErrors() {
				diagnostics = diagnostics.Extend(diags)
				continue
			}
			return variables, nil
		}
	}

	return nil, diagnostics
}

package action

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Action interface {
	GetName() string
	Dependencies() []hcl.Traversal
	Run() hcl.Diagnostics
	Preload(ctx *hcl.EvalContext) hcl.Diagnostics

	// Settle forces evaluation of expressions that depend on other
	// actions
	// Settle() hcl.Diagnostics

	Addressable
}

type Addressable interface {
	Path() cty.Path
}

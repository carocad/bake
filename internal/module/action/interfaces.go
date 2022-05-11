package action

import (
	"bake/internal/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Action interface {
	Apply() hcl.Diagnostics
}

type Address interface {
	GetName() string
	Path() cty.Path
	Dependencies() []hcl.Traversal
	Plan(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics)

	values.Cty
}

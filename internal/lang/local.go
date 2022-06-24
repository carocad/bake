package lang

import (
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Local struct {
	name  string
	expr  hcl.Expression
	value cty.Value
}

func (local Local) GetFilename() string {
	return local.expr.Range().Filename
}

func (local Local) GetPath() cty.Path {
	return cty.GetAttrPath(schema.LocalScope).GetAttr(local.name)
}

func (local Local) Dependencies() ([]hcl.Traversal, hcl.Diagnostics) {
	return local.expr.Variables(), nil
}

func (local Local) Decode(ctx *hcl.EvalContext) (Action, hcl.Diagnostics) {
	newLocal := local
	value, diagnostics := local.expr.Value(ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	newLocal.value = value
	return newLocal, nil
}

func (local Local) Apply(state *config.State) *sync.WaitGroup {
	return nil
}

func (local Local) CTY() cty.Value {
	return local.value
}

func (local Local) Hash() []config.Hash {
	return nil
}

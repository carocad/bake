package action

import (
	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Phony struct {
	Task
}

func NewPhony(block *hcl.Block, ctx *hcl.EvalContext) (Address, hcl.Diagnostics) {
	content, diagnostics := block.Body.Content(lang.PhonySchema())
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	task, diagnostics := NewTask(block.Labels[0], content.Attributes, ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return &Phony{*task}, nil
}

func (phony Phony) CTY() cty.Value {
	return CtyTask(phony)
}

func (phony Phony) Path() cty.Path {
	return cty.GetAttrPath(lang.PhonyLabel).GetAttr(phony.Name)
}

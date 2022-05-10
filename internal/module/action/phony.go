package action

import (
	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Phony struct {
	Task
}

func (phony Phony) Path() cty.Path {
	return cty.GetAttrPath(lang.PhonyLabel).GetAttr(phony.Name)
}

func NewPhony(block *hcl.Block, ctx *hcl.EvalContext) (Action, hcl.Diagnostics) {
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

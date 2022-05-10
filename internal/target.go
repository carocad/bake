package internal

import (
	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type Target struct {
	Task
	Filename string
	Sources  []string
}

func (target Target) Path() cty.Path {
	return cty.GetAttrPath(target.Name)
}

func NewTarget(block *hcl.Block, ctx *hcl.EvalContext) (Action, hcl.Diagnostics) {
	content, diags := block.Body.Content(lang.TargetSchema())
	if diags.HasErrors() {
		return nil, diags
	}

	task, diagnostics := NewTask(block.Labels[0], content.Attributes, ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	var filename string
	diags = gohcl.DecodeExpression(content.Attributes[lang.FilenameAttr].Expr, ctx, &filename)
	if diags.HasErrors() {
		return nil, diags
	}

	sources := make([]string, 0)
	diags = gohcl.DecodeExpression(content.Attributes[lang.SourcesAttr].Expr, ctx, &sources)
	if diags.HasErrors() {
		return nil, diags
	}

	return &Target{
		Task:     *task,
		Filename: filename,
		Sources:  sources,
	}, nil

}

package action

import (
	"bake/internal/lang"
	"bake/internal/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type Target struct {
	Task
	Filename     values.EventualString
	filenameExpr hcl.Expression
	Sources      values.EventualStringSlice
	sourcesExpr  hcl.Expression
}

func NewTarget(block *hcl.Block, ctx *hcl.EvalContext) (Address, hcl.Diagnostics) {
	content, diags := block.Body.Content(lang.TargetSchema())
	if diags.HasErrors() {
		return nil, diags
	}

	task, diagnostics := NewTask(block.Labels[0], content.Attributes, ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	var filenameExpr hcl.Expression
	if attr, ok := content.Attributes[lang.FilenameAttr]; ok {
		filenameExpr = attr.Expr
	}

	var sourcesExpr hcl.Expression
	if attr, ok := content.Attributes[lang.SourcesAttr]; ok {
		sourcesExpr = attr.Expr
	}

	return &Target{
		Task:         *task,
		filenameExpr: filenameExpr,
		sourcesExpr:  sourcesExpr,
	}, nil
}

func (target Target) CTY() cty.Value {
	return CtyTask(target)
}

func (target Target) Path() cty.Path {
	return cty.GetAttrPath(target.Name)
}

func (target Target) Plan(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics) {
	var filename string
	diags := gohcl.DecodeExpression(target.filenameExpr, ctx, &filename)
	if diags.HasErrors() {
		return nil, diags
	}

	target.Filename = values.EventualString{
		String: filename,
		Valid:  true,
	}

	sources := make([]string, 0)
	diags = gohcl.DecodeExpression(target.sourcesExpr, ctx, &sources)
	if diags.HasErrors() {
		return nil, diags
	}

	target.Sources = values.EventualStringSlice{
		Slice: sources,
		Valid: true,
	}

	return target.Task.Plan(ctx)
}

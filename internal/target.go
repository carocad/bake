package internal

import (
	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
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

func NewTarget(block *hcl.Block, ctx *hcl.EvalContext) (*Target, hcl.Diagnostics) {
	content, diags := block.Body.Content(lang.TargetSchema())
	if diags.HasErrors() {
		return nil, diags
	}

	task, diagnostics := NewTask(block.Labels[0], content.Attributes, ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	filename, diagnostics := lang.String(lang.FilenameAttr, content.Attributes, ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	sources, diagnostics := lang.ListOfStrings(lang.SourcesAttr, content.Attributes, ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return &Target{
		Task:     *task,
		Filename: filename,
		Sources:  sources,
	}, nil

}

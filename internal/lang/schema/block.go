package schema

import (
	"github.com/hashicorp/hcl/v2"
)

func GetRangeFor(block *hcl.Block, name string) *hcl.Range {
	attributes, diagnostics := block.Body.JustAttributes()
	if diagnostics.HasErrors() {
		return nil
	}

	for _, attribute := range attributes {
		if attribute.Name == name {
			return attribute.Expr.Range().Ptr()
		}
	}

	return nil
}

func TupleOfReferences(attr *hcl.Attribute) ([]hcl.Traversal, hcl.Diagnostics) {
	var references []hcl.Traversal
	exprs, diags := hcl.ExprList(attr.Expr)

	for _, expr := range exprs {
		traversal, travDiags := hcl.AbsTraversalForExpr(expr)
		diags = append(diags, travDiags...)
		if len(traversal) != 0 {
			references = append(references, traversal)
		}
	}

	return references, diags
}

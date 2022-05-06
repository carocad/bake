package lang

import "github.com/hashicorp/hcl/v2"

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

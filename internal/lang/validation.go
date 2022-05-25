package lang

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

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

func checkDependsOn(body hcl.Body) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return diags
	}

	for _, attr := range attrs {
		if attr.Name == DependsOnAttr {
			_, diags := TupleOfReferences(attrs[DependsOnAttr])
			return diags
		}

		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Unsupported argument",
			Detail:   fmt.Sprintf(`An argument named "%s" is not expected here`, attr.Name),
			Subject:  attr.Expr.Range().Ptr(),
		}}
	}

	return nil
}

package lang

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
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

func String(name string, attributes hcl.Attributes, ctx *hcl.EvalContext) (string, hcl.Diagnostics) {
	attr, found := attributes[name]
	if !found {
		return "", nil
	}

	value, diagnostics := attr.Expr.Value(ctx)
	if diagnostics.HasErrors() {
		return "", diagnostics
	}

	if value.Type() != cty.String {
		return "", hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("'%s' must be a string", name),
			Subject:  attr.Expr.Range().Ptr(),
		}}
	}

	if value.IsNull() {
		return "", hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("'%s' cannot be null", name),
			Subject:  attr.Expr.Range().Ptr(),
		}}
	}

	if !value.IsKnown() {
		return "", hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "no interpolation is allowed here ",
			Subject:  attr.Expr.Range().Ptr(),
		}}
	}

	return value.AsString(), nil
}

func ListOfStrings(name string, attributes hcl.Attributes, ctx *hcl.EvalContext) ([]string, hcl.Diagnostics) {
	attr, found := attributes[name]
	if !found {
		return make([]string, 0), nil
	}

	value, diagnostics := attr.Expr.Value(ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	if value.Type() != cty.List(cty.String) {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("'%s' must be a list of strings", name),
			Subject:  attr.Expr.Range().Ptr(),
		}}
	}

	result := make([]string, 0)
	for _, element := range value.AsValueSlice() {
		if element.IsNull() {
			return nil, hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  fmt.Sprintf("'%s' cannot be null", name),
				Subject:  attr.Expr.Range().Ptr(),
			}}
		}

		if !element.IsKnown() {
			return nil, hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "no interpolation is allowed here ",
				Subject:  attr.Expr.Range().Ptr(),
			}}
		}

		result = append(result, element.AsString())
	}

	return result, nil
}

package schema

import (
	"bake/internal/concurrent"
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/convert"
	"github.com/zclconf/go-cty/cty/gocty"
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

func ForEachEntries(block *hcl.Block, ctx *hcl.EvalContext) (map[string]string, hcl.Diagnostics) {
	attributes, diags := block.Body.JustAttributes()
	if diags.HasErrors() {
		return nil, diags
	}

	for name, attr := range attributes {
		if name == ForEachAttr {
			value, diags := attr.Expr.Value(ctx)
			if diags.HasErrors() {
				return nil, diags
			}

			forEachSet := make([]string, 0)
			err := gocty.FromCtyValue(value, &forEachSet)
			if err == nil {
				return concurrent.SetToMap(forEachSet), nil
			}

			diagnostic := hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary: fmt.Sprintf(
					`"for_each" field must be either a set or a map of strings but "%s" was provided`,
					value.Type().FriendlyNameForConstraint(),
				),
				Detail:      err.Error(),
				Subject:     attr.Expr.Range().Ptr(),
				Context:     &block.DefRange,
				Expression:  attr.Expr,
				EvalContext: ctx,
			}}
			forEachMap := make(map[string]string)
			// somehow FromCtyValue doesnt do this convertion itself :/
			if value.Type().IsObjectType() {
				v2, err := convert.Convert(value, cty.Map(cty.String))
				if err != nil {
					return nil, diagnostic
				}

				value = v2
			}

			err = gocty.FromCtyValue(value, &forEachMap)
			if err != nil {
				return nil, diagnostic
			}

			return forEachMap, nil
		}
	}

	return nil, nil
}

// ValidateAttributes checks that a remaining body only contains
// depends_on and for_each attributes
func ValidateAttributes(body hcl.Body) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return diags
	}

	for _, attr := range attrs {
		if attr.Name == DependsOnAttr {
			_, diags := TupleOfReferences(attrs[DependsOnAttr])
			if diags.HasErrors() {
				return diags
			}

			continue
		}

		if attr.Name == ForEachAttr {
			continue
		}

		// only depends on is allowed
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "Unsupported argument",
			Detail:   fmt.Sprintf(`An argument named "%s" is not expected here`, attr.Name),
			Subject:  attr.Expr.Range().Ptr(),
		}}
	}

	return nil
}

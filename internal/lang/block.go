package lang

import "github.com/hashicorp/hcl/v2"

func getCommandRange(block *hcl.Block) *hcl.Range {
	attributes, diagnostics := block.Body.JustAttributes()
	if diagnostics.HasErrors() {
		return nil
	}

	for _, attribute := range attributes {
		if attribute.Name == CommandAttr {
			return attribute.Expr.Range().Ptr()
		}
	}

	return nil
}

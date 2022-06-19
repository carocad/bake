package paths

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func FromTraversal(traversal hcl.Traversal) cty.Path {
	path := cty.Path{}
	for _, step := range traversal {
		switch traverse := step.(type) {
		case hcl.TraverseRoot:
			path = path.GetAttr(traverse.Name)
		case hcl.TraverseAttr:
			path = path.GetAttr(traverse.Name)
		case hcl.TraverseIndex:
			path = path.Index(traverse.Key)
		default:
			// fail fast -> implement more cases as bake evolves
			panic(fmt.Sprintf("unknown traversal step '%s'", traverse))
		}
	}

	return path
}

func String(path cty.Path) string {
	result := ""
	for _, step := range path {
		ss := StepString(step)
		// edge case for first entry
		if _, ok := step.(cty.GetAttrStep); ok && result != "" {
			ss = "." + ss
		}

		result += ss
	}

	return result
}

func StepString(step cty.PathStep) string {
	switch ss := step.(type) {
	case cty.GetAttrStep:
		return ss.Name
	case cty.IndexStep:
		switch ss.Key.Type() {
		case cty.Number:
			return fmt.Sprintf(`[%d]`, ss.Key.AsBigFloat())
		case cty.String:
			return fmt.Sprintf(`["%s"]`, ss.Key.AsString())
		}
	}

	// maybe go-cty added a new step type?
	panic("key value not number or string")
}

func Type(step cty.PathStep) cty.Type {
	if _, ok := step.(cty.GetAttrStep); ok {
		return cty.String
	}

	if attr, ok := step.(cty.IndexStep); ok {
		return attr.Key.Type()
	}

	// maybe go-cty added a new step type?
	panic(fmt.Sprintf("unknown step: %#v", step))
}

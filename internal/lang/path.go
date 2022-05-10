package lang

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

func ToPath(traversal hcl.Traversal) cty.Path {
	path := cty.Path{}
	for _, step := range traversal {
		switch traverse := step.(type) {
		case hcl.TraverseRoot:
			path = path.GetAttr(traverse.Name)
		case hcl.TraverseAttr:
			path = path.GetAttr(traverse.Name)
		default:
			// fail fast -> implement more cases as bake evolves
			panic(fmt.Sprintf("unknown traversal step '%s'", traverse))
		}
	}

	return path
}

func PathString(path cty.Path) string {
	result := ""
	for _, step := range path {
		switch ss := step.(type) {
		case cty.GetAttrStep:
			if result == "" {
				result = ss.Name
			} else {
				result += "." + ss.Name
			}
		case cty.IndexStep:
			switch ss.Key.Type() {
			case cty.Number:
				// todo: is this ok?
				result += fmt.Sprintf("[%d]", ss.Key.AsBigFloat())
			case cty.String:
				result += fmt.Sprintf("[%s]", ss.Key.AsString())
			default:
				// this implies a 🐞 in the code
				panic("key value not number or string")
			}
		}
	}

	return result
}

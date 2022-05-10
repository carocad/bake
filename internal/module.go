package internal

import (
	"fmt"
	"path/filepath"

	"bake/internal/lang"
	"bake/internal/lang/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Module struct {
	Actions []Action
	// name by which the module is known; by convention the root module
	// doesn't have a name as it is "global"
	name string
	cwd  string
}

func NewRootModule(cwd string) *Module {
	return NewModule("", cwd)
}

func NewModule(name, cwd string) *Module {
	return &Module{name: name, cwd: cwd, Actions: make([]Action, 0)}
}

func (module Module) Path() cty.Path {
	// root module
	if module.name == "" {
		return cty.Path{}
	}

	return cty.GetAttrPath("module").GetAttr(module.name)
}

func (module *Module) GetContent(file *hcl.File, filename string) hcl.Diagnostics {
	content, diags := file.Body.Content(lang.RecipeSchema())
	if diags.HasErrors() {
		return diags
	}

	context, diags := module.EvalContext(filename)
	if diags.HasErrors() {
		return diags
	}

	for _, block := range content.Blocks {
		switch block.Type {
		case lang.PhonyLabel:
			action, diags := NewPhony(block, context)
			if diags.HasErrors() {
				return diags
			}
			module.Actions = append(module.Actions, action)
		case lang.TargetLabel:
			action, diags := NewTarget(block, context)
			if diags.HasErrors() {
				return diags
			}
			module.Actions = append(module.Actions, action)
		}
	}

	// todo: loop over all actions again and "settle" the unknown vars
	/*
		context, diags = module.EvalContext(filename)
		if diags.HasErrors() {
			return diags
		}
	*/
	return nil
}

func (module Module) EvalContext(filename string) (*hcl.EvalContext, hcl.Diagnostics) {
	ctx := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(module.cwd),
			"module":  cty.StringVal(filepath.Join(module.cwd, filepath.Dir(filename))),
			"current": cty.StringVal(filepath.Join(module.cwd, filename)),
		}),
	}

	phonyPrefix := cty.GetAttrPath(lang.PhonyLabel)
	phony := map[string]cty.Value{}
	for _, action := range module.Actions {
		name := action.GetName()
		path := action.Path()
		if path.HasPrefix(phonyPrefix) {
			phony[name] = cty.ObjectVal(values.CTY(action))
			continue
		}
		// only targets for now !!
		ctx[name] = cty.ObjectVal(values.CTY(action))
	}
	ctx[lang.PhonyLabel] = cty.ObjectVal(phony)

	return &hcl.EvalContext{
		Variables: ctx,
		Functions: lang.Functions(),
	}, nil
}

type mark int

const (
	unmarked = iota
	temporary
	permanent
)

type actionMark struct {
	Action
	mark
}

// Dependencies according to
// https://www.wikiwand.com/en/Topological_sorting#/Depth-first_search
// NOTE: the task itself is the last element of the dependency list
func (module Module) Dependencies(task Action) ([]Action, hcl.Diagnostics) {
	markers := make(map[string]*actionMark)
	path := PathString(task.Path())
	markers[path] = &actionMark{task, unmarked}
	order, diags := module.visit(path, markers)
	if diags.HasErrors() {
		return nil, diags
	}

	return order, nil
}

const cyclicalDependency = "cyclical dependency detected"

func (module Module) visit(current string, markers map[string]*actionMark) ([]Action, hcl.Diagnostics) {
	id := markers[current]
	if id.mark == permanent {
		return nil, nil
	}

	if id.mark == temporary {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  cyclicalDependency,
			Detail:   current,
		}}
	}

	id.mark = temporary
	order := make([]Action, 0)
	for _, dep := range id.Dependencies() {
		innerID, diags := module.GetByID(dep)
		if diags.HasErrors() {
			return nil, diags
		}

		// make sure we initialize the marker
		path := PathString(innerID.Path())
		if _, found := markers[path]; !found {
			markers[path] = &actionMark{innerID, unmarked}
		}

		inner, diags := module.visit(path, markers)
		if diags.HasErrors() {
			for _, diag := range diags {
				if diag.Summary == cyclicalDependency {
					diag.Detail = fmt.Sprintf("%s -> %s", current, diag.Detail)
				}
			}
			return nil, diags
		}

		order = append(order, inner...)
	}
	id.mark = permanent
	order = append(order, id.Action)
	return order, nil
}

func (module Module) GetByID(traversal hcl.Traversal) (Action, hcl.Diagnostics) {
	path := ToPath(traversal)
	for _, action := range module.Actions {
		if action.Path().Equals(path) {
			return action, nil
		}
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "unknown reference",
		Subject:  traversal.SourceRange().Ptr(),
	}}
}

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

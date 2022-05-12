package module

import (
	"bake/internal/module/action"
	"github.com/zclconf/go-cty/cty"
)

type Module struct {
	fileAddresses map[string][]action.Address
	// name by which the module is known; by convention the root module
	// doesn't have a name as it is "global"
	name string
	cwd  string
}

func NewRootModule(cwd string) *Module {
	return NewModule("", cwd)
}

func NewModule(name, cwd string) *Module {
	return &Module{name: name, cwd: cwd, fileAddresses: map[string][]action.Address{}}
}

func (module Module) Path() cty.Path {
	// root module
	if module.name == "" {
		return cty.Path{}
	}

	return cty.GetAttrPath("module").GetAttr(module.name)
}

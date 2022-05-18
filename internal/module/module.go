package module

import (
	"fmt"

	"bake/internal/functional"
	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Module struct {
	// name by which the module is known; by convention the root module
	// doesn't have a name as it is "global"
	name string
	cwd  string
}

func NewRootModule(cwd string) *Module {
	return NewModule("", cwd)
}

func NewModule(name, cwd string) *Module {
	return &Module{name: name, cwd: cwd}
}

func (module Module) Path() cty.Path {
	// root module
	if module.name == "" {
		return cty.Path{}
	}

	return cty.GetAttrPath("module").GetAttr(module.name)
}

func (module Module) GetTask(name string, addresses []lang.RawAddress) (lang.RawAddress, hcl.Diagnostics) {
	for _, address := range addresses {
		if lang.PathString(address.GetPath()) != name {
			continue
		}

		return address, nil
	}

	options := functional.Map(addresses, lang.AddressToString[lang.RawAddress])
	suggestion := functional.Suggest(name, options)
	summary := "couldn't find any target with name " + name
	if suggestion != "" {
		summary += fmt.Sprintf(`. Did you mean "%s"`, suggestion)
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  summary,
	}}
}

package module

import (
	"bake/internal/functional"
	"bake/internal/lang"
	"fmt"

	"github.com/hashicorp/hcl/v2"
)

func GetTask(name string, addresses []lang.RawAddress) (lang.RawAddress, hcl.Diagnostics) {
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

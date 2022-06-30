package internal

import (
	"fmt"
	"io/ioutil"
	"path/filepath"

	"bake/internal/lang"
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"bake/internal/module"
	"bake/internal/util"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func ReadRecipes(cwd string, parser *hclparse.Parser) ([]config.RawAddress, hcl.Diagnostics) {
	files, err := ioutil.ReadDir(cwd)
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "couldn't read files in " + cwd,
			Detail:   err.Error(),
		}}
	}

	addresses := make([]config.RawAddress, 0)
	for _, filename := range files {
		if filepath.Ext(filename.Name()) != ".hcl" { // todo: change to .rcp
			continue
		}

		// read the file but don't decode it yet
		f, diags := parser.ParseHCLFile(filename.Name())
		if diags.HasErrors() {
			return nil, diags
		}

		content, diags := f.Body.Content(schema.FileSchema())
		if diags.HasErrors() {
			return nil, diags
		}

		for _, block := range content.Blocks {
			address, diagnostics := lang.NewPartialAddress(block)
			if diagnostics.HasErrors() {
				return nil, diagnostics
			}
			addresses = append(addresses, address...)
		}
	}

	return addresses, nil
}

func Do(taskName string, state *config.State, addrs []config.RawAddress) hcl.Diagnostics {
	task, diags := getTask(taskName, addrs)
	if diags.HasErrors() {
		return diags
	}

	coordinator := module.NewCoordinator()
	actions, diags := coordinator.Do(state, task, addrs)
	if state.Flags.Dry || state.Flags.Prune {
		return diags
	}

	for _, action := range actions {
		state.Lock.Update(action)
	}

	err := state.Lock.Store(state.CWD)
	if err != nil {
		diags.Append(&hcl.Diagnostic{
			Severity: hcl.DiagError,
			Summary:  "error storing state",
			Detail:   err.Error(),
		})
	}

	return diags
}

func getTask(name string, addresses []config.RawAddress) (config.RawAddress, hcl.Diagnostics) {
	for _, address := range addresses {
		if config.AddressToString(address) != name {
			continue
		}

		return address, nil
	}

	options := util.Map(addresses, config.AddressToString[config.RawAddress])
	suggestion := util.Suggest(name, options)
	summary := "couldn't find any target with name " + name
	if suggestion != "" {
		summary += fmt.Sprintf(`. Did you mean "%s"`, suggestion)
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  summary,
	}}
}

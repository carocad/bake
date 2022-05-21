package internal

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"bake/internal/lang"
	"bake/internal/module"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

type System struct {
	parser *hclparse.Parser
	cwd    string
}

func NewSystem() (*System, hcl.Diagnostics) {
	// create a parser
	parser := hclparse.NewParser()
	// where are we?
	cwd, err := os.Getwd()
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "couldn't get current working directory",
			Detail:   err.Error(),
		}}
	}

	return &System{
		parser: parser,
		cwd:    cwd,
	}, nil
}

func (state System) NewLogger() hcl.DiagnosticWriter {
	return hcl.NewDiagnosticTextWriter(os.Stdout, state.parser.Files(), 78, true)
}

func (state System) readRecipes() ([]lang.RawAddress, hcl.Diagnostics) {
	files, err := ioutil.ReadDir(state.cwd)
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "couldn't read files in " + state.cwd,
			Detail:   err.Error(),
		}}
	}

	addresses := make([]lang.RawAddress, 0)
	for _, filename := range files {
		if filepath.Ext(filename.Name()) != ".hcl" { // todo: change to .rcp
			continue
		}

		// read the file but don't decode it yet
		f, diags := state.parser.ParseHCLFile(filename.Name())
		if diags.HasErrors() {
			return nil, diags
		}

		content, diags := f.Body.Content(lang.RecipeSchema())
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

func (state System) Do(target string, eval lang.ContextData) ([]lang.Action, hcl.Diagnostics) {
	addrs, diags := state.readRecipes()
	if diags.HasErrors() {
		return nil, diags
	}

	task, diags := module.GetTask(target, addrs)
	if diags.HasErrors() {
		return nil, diags
	}

	coordinator := module.NewCoordinator(context.TODO(), eval)
	actions, diags := coordinator.Do(task, addrs)
	if diags.HasErrors() {
		return nil, diags
	}

	return actions, nil
}

func (state System) Plan(target string) hcl.Diagnostics {
	eval := lang.NewContextData(state.cwd, true)
	_, diags := state.Do(target, eval)
	return diags
}

func (state System) Apply(target string) hcl.Diagnostics {
	eval := lang.NewContextData(state.cwd, false)
	_, diags := state.Do(target, eval)
	return diags
}

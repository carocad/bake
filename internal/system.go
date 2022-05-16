package internal

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"bake/internal/lang"
	"bake/internal/module"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

type System struct {
	root   *module.Module // the root module
	parser *hclparse.Parser
	cwd    string
	Logger hcl.DiagnosticWriter
}

func NewSystem() (*System, hcl.Diagnostics) {
	// create a parser
	parser := hclparse.NewParser()
	// setup a pretty printer for errors
	logger := hcl.NewDiagnosticTextWriter(os.Stdout, parser.Files(), 78, true)
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
		root:   module.NewRootModule(cwd),
		parser: parser,
		Logger: logger,
		cwd:    cwd,
	}, nil
}

func (state System) readRecipes() (map[string][]lang.RawAddress, hcl.Diagnostics) {
	fileAddresses := map[string][]lang.RawAddress{}
	files, err := ioutil.ReadDir(state.cwd)
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "couldn't read files in " + state.cwd,
			Detail:   err.Error(),
		}}
	}

	for _, filename := range files {
		if filepath.Ext(filename.Name()) != ".hcl" { // todo: change to .rcp
			continue
		}

		// read the file but don't decode it yet
		f, diags := state.parser.ParseHCLFile(filename.Name())
		if diags.HasErrors() {
			return nil, diags
		}

		addrs, diags := state.root.GetContent(f)
		if diags.HasErrors() {
			return nil, diags
		}

		fileAddresses[filename.Name()] = addrs
	}

	return fileAddresses, nil
}

func (state System) Plan(target string) ([]lang.Action, hcl.Diagnostics) {
	addrs, diags := state.readRecipes()
	if diags.HasErrors() {
		return nil, diags
	}

	actions, diags := state.root.Plan(target, addrs)
	if diags.HasErrors() {
		return nil, diags
	}

	return actions, nil
}

func (state System) Apply(action string) hcl.Diagnostics {
	actions, diags := state.Plan(action)
	if diags.HasErrors() {
		return diags
	}

	for _, act := range actions {
		_, diags = act.Apply()
		if diags.HasErrors() {
			return diags
		}
	}

	return nil
}

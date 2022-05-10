package internal

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

type System struct {
	root   *Module // the root module
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
		root:   NewRootModule(cwd),
		parser: parser,
		Logger: logger,
		cwd:    cwd,
	}, nil
}

func (state System) readRecipes() hcl.Diagnostics {
	files, err := ioutil.ReadDir(state.cwd)
	if err != nil {
		return hcl.Diagnostics{{
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
			return diags
		}

		diags = state.root.GetContent(f, filename.Name())
		if diags.HasErrors() {
			return diags
		}
	}

	return nil
}

func (state System) Plan(target string) ([]Action, hcl.Diagnostics) {
	diags := state.readRecipes()
	if diags.HasErrors() {
		return nil, diags
	}

	// todo: add missing targets here
	for _, action := range state.root.Actions {
		if action.GetName() == target {
			deps, diags := state.root.Dependencies(action)
			if diags.HasErrors() {
				return nil, diags
			}

			return deps, nil
		}
	}

	return nil, hcl.Diagnostics{{
		Severity: hcl.DiagError,
		Summary:  "couldn't find any target with name " + target,
	}}
}

func (state System) Apply(action string) hcl.Diagnostics {
	actions, diags := state.Plan(action)
	if diags.HasErrors() {
		return diags
	}

	// todo: defer state saving
	for _, action := range actions {
		log.Println("executing " + action.GetName())
		diags = action.Run()
		if diags.HasErrors() {
			return diags
		}
	}

	return nil
}

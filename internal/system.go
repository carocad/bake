package internal

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"

	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
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
		root:   &Module{},
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

		// decode the Module into a Go Struct
		var recipe lang.Recipe
		evalContext, diags := state.EvalContext(filename.Name())
		if diags.HasErrors() {
			return diags
		}

		diags = gohcl.DecodeBody(f.Body, evalContext, &recipe)
		if diags.HasErrors() {
			return diags
		}

		// todo: move to its own method?
		for _, langPhony := range recipe.Phonies {
			phony, diags := NewPhony(langPhony)
			if diags.HasErrors() {
				return diags
			}

			state.root.Actions = append(state.root.Actions, phony)
		}

		for _, langTarget := range recipe.Targets {
			target, diags := NewTarget(langTarget)
			if diags.HasErrors() {
				return diags
			}

			state.root.Actions = append(state.root.Actions, target)
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

func (state System) EvalContext(filename string) (*hcl.EvalContext, hcl.Diagnostics) {
	ctx := map[string]cty.Value{
		"pid": cty.NumberIntVal(int64(os.Getpid())),
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(state.cwd),
			"module":  cty.StringVal(filepath.Join(state.cwd, filepath.Dir(filename))),
			"current": cty.StringVal(filepath.Join(state.cwd, filename)),
		}),
	}

	for _, action := range state.root.Actions {
		ctx[action.GetName()] = cty.ObjectVal(Value(action))
	}

	return &hcl.EvalContext{
		Variables: ctx,
		Functions: lang.Functions(),
	}, nil
}

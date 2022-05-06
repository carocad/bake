package main

import (
	"log"
	"os"
	"path/filepath"

	"bake/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/zclconf/go-cty/cty"
)

const recipeFile = "recipe.hcl"

func main() {
	// create a parser
	parser := hclparse.NewParser()
	// setup a pretty printer for errors
	logger := hcl.NewDiagnosticTextWriter(os.Stdout, parser.Files(), 78, true)
	// read the file but don't decode it yet
	f, diags := parser.ParseHCLFile(recipeFile)
	if diags.HasErrors() {
		LogAndExit(logger, diags)
	}

	// decode the AST into a Go Struct
	var langRecipe lang.Recipe
	evalContext, diags := EvalContext()
	if diags.HasErrors() {
		LogAndExit(logger, diags)
	}

	diags = gohcl.DecodeBody(f.Body, evalContext, &langRecipe)
	if diags.HasErrors() {
		LogAndExit(logger, diags)
	}

	recipe, diags := Decode(langRecipe)
	if diags.HasErrors() {
		LogAndExit(logger, diags)
	}

	deps, diags := recipe.Dependencies(recipe.Phonies[0])
	logger.WriteDiagnostics(diags.Extend(diags))
	log.Printf("Dependencies are %#v", deps)
}

// Decode todo: check that the IDs are not duplicated
func Decode(container lang.Recipe) (*Recipe, hcl.Diagnostics) {
	diagnostics := make(hcl.Diagnostics, 0)
	recipe := Recipe{}
	for _, langPhony := range container.Phonies {
		phony, diags := NewPhony(langPhony)
		if diags.HasErrors() {
			diagnostics = append(diagnostics, diags...)
			continue
		}

		recipe.Phonies = append(recipe.Phonies, *phony)
	}

	for _, langTarget := range container.Targets {
		target, diags := NewTarget(langTarget)
		if diags.HasErrors() {
			diagnostics = append(diagnostics, diags...)
			continue
		}

		recipe.Targets = append(recipe.Targets, *target)
	}

	return &recipe, diagnostics
}

// EvalContext provides an evaluation context so that special variables
// are available to the recipe user
func EvalContext() (*hcl.EvalContext, hcl.Diagnostics) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, hcl.Diagnostics{
			&hcl.Diagnostic{
				Severity: hcl.DiagError,
				Summary:  "couldn't get current working directory",
				Detail:   "this is an internal 'bake' error. Please contact the chef",
			},
		}
	}

	return &hcl.EvalContext{
		Variables: map[string]cty.Value{
			"pid": cty.NumberIntVal(int64(os.Getpid())),
			"path": cty.ObjectVal(map[string]cty.Value{
				"root":    cty.StringVal(cwd),
				"module":  cty.StringVal(filepath.Join(cwd, recipeFile)),
				"current": cty.StringVal(cwd),
			}),
		},
		Functions: lang.Functions(),
	}, nil
}

func LogAndExit(logger hcl.DiagnosticWriter, diags hcl.Diagnostics) {
	logger.WriteDiagnostics(diags)
	os.Exit(1)
}

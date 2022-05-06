package main

import (
	"log"
	"os"

	"bake/internal"
	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
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
	evalContext, diags := internal.EvalContext()
	if diags.HasErrors() {
		LogAndExit(logger, diags)
	}

	diags = gohcl.DecodeBody(f.Body, evalContext, &langRecipe)
	if diags.HasErrors() {
		LogAndExit(logger, diags)
	}

	recipe, diags := internal.Decode(langRecipe)
	if diags.HasErrors() {
		LogAndExit(logger, diags)
	}

	deps, diags := recipe.Dependencies(recipe.Phonies[0])
	logger.WriteDiagnostics(diags.Extend(diags))
	log.Printf("Dependencies are %#v", deps)
}

func LogAndExit(logger hcl.DiagnosticWriter, diags hcl.Diagnostics) {
	logger.WriteDiagnostics(diags)
	os.Exit(1)
}

package main

import (
	"bake/internal"
	"bake/internal/state"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func main() {
	// create a parser
	parser := hclparse.NewParser()
	// logger for diagnostics
	log := hcl.NewDiagnosticTextWriter(os.Stdout, parser.Files(), 78, true)
	// where are we?
	cwd, err := os.Getwd()
	if err != nil {
		Fatal(log, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "couldn't get current working directory",
			Detail:   err.Error(),
		}})
	}

	addrs, diags := internal.ReadRecipes(cwd, parser)
	if err != nil {
		Fatal(log, diags)
	}

	config := state.NewConfig(cwd)
	config.Task = "main" // TODO
	diags = internal.Do(config, addrs)
	if diags.HasErrors() {
		Fatal(log, diags)
	}
}

func Fatal(log hcl.DiagnosticWriter, diagnostics hcl.Diagnostics) {
	log.WriteDiagnostics(diagnostics)
	os.Exit(1)
}

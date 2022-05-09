package main

import (
	"os"

	"bake/internal"
	"github.com/hashicorp/hcl/v2"
)

func main() {
	state, diags := internal.NewSystem()
	if diags.HasErrors() {
		state.Logger.WriteDiagnostics(diags)
		os.Exit(1)
	}

	diags = state.Apply("main")
	if diags.HasErrors() {
		state.Logger.WriteDiagnostics(diags)
		os.Exit(1)
	}
}

func LogAndExit(logger hcl.DiagnosticWriter, diags hcl.Diagnostics) {
	logger.WriteDiagnostics(diags)
	os.Exit(1)
}

package main

import (
	"fmt"
	"os"

	"bake/internal"
)

func main() {
	state, diags := internal.NewSystem()
	if diags.HasErrors() {
		fmt.Print(diags.Error())
		os.Exit(1)
	}

	logger := state.NewLogger()
	diags = state.Apply("main")
	if diags.HasErrors() {
		logger.WriteDiagnostics(diags)
		os.Exit(1)
	}
}

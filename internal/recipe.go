package internal

import (
	"os"
	"path/filepath"

	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

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
				"module":  cty.StringVal(filepath.Join(cwd, "TODO")),
				"current": cty.StringVal(cwd),
			}),
		},
		Functions: lang.Functions(),
	}, nil
}

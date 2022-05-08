package internal

import (
	"os"
	"path/filepath"

	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Recipe struct {
	Phonies []Phony
	Targets []Target
}

// NewRecipe todo: check that the task.Names are not duplicated
func NewRecipe(container lang.Recipe) (*Recipe, hcl.Diagnostics) {
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

func (recipe Recipe) EvalContext() (*hcl.EvalContext, hcl.Diagnostics) {
	ctx := map[string]cty.Value{}
	for _, phony := range recipe.Phonies {
		ctx[phony.Name] = cty.ObjectVal(Value(phony))
	}

	for _, target := range recipe.Targets {
		ctx[target.Name] = cty.ObjectVal(Value(target))
	}

	return &hcl.EvalContext{Variables: ctx}, nil
}

// StaticEvalContext provides an evaluation context so that special variables
// are available to the recipe user
func StaticEvalContext() (*hcl.EvalContext, hcl.Diagnostics) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "couldn't get current working directory",
			Detail:   err.Error(),
		}}
	}

	return &hcl.EvalContext{
		// todo: load all tasks here
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

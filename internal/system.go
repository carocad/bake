package internal

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	"bake/internal/lang"
	"bake/internal/module"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
)

func ReadRecipes(cwd string, parser *hclparse.Parser) ([]lang.RawAddress, hcl.Diagnostics) {
	files, err := ioutil.ReadDir(cwd)
	if err != nil {
		return nil, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "couldn't read files in " + cwd,
			Detail:   err.Error(),
		}}
	}

	addresses := make([]lang.RawAddress, 0)
	for _, filename := range files {
		if filepath.Ext(filename.Name()) != ".hcl" { // todo: change to .rcp
			continue
		}

		// read the file but don't decode it yet
		f, diags := parser.ParseHCLFile(filename.Name())
		if diags.HasErrors() {
			return nil, diags
		}

		content, diags := f.Body.Content(lang.FileSchema())
		if diags.HasErrors() {
			return nil, diags
		}

		for _, block := range content.Blocks {
			address, diagnostics := lang.NewPartialAddress(block)
			if diagnostics.HasErrors() {
				return nil, diagnostics
			}
			addresses = append(addresses, address...)
		}
	}

	return addresses, nil
}

func Do(taskName string, state *lang.State, addrs []lang.RawAddress) hcl.Diagnostics {
	task, diags := module.GetTask(taskName, addrs)
	if diags.HasErrors() {
		return diags
	}

	coordinator := module.NewCoordinator(context.TODO(), *state)
	start := time.Now()
	actions, diags := coordinator.Do(task, addrs)
	end := time.Now()
	fmt.Printf("\ndone in %s\n", end.Sub(start).String())

	if !state.Flags.Dry && !state.Flags.Prune {
		state.Lock.Update(actions)
		err := state.Lock.Store(state.CWD)
		if err != nil {
			state.NewLogger(task).Fatal(fmt.Errorf("error storing state: %w", err))
		}
	}

	return diags
}

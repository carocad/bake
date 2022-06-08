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

func Do(taskName string, config *lang.State, addrs []lang.RawAddress) hcl.Diagnostics {
	task, diags := module.GetTask(taskName, addrs)
	if diags.HasErrors() {
		return diags
	}

	coordinator := module.NewCoordinator(context.TODO(), *config)
	log := config.NewLogger(task)
	start := time.Now()
	actions, diags := coordinator.Do(task, addrs)
	end := time.Now()
	log.Printf(`done in %s`, end.Sub(start).String())
	if diags.HasErrors() {
		return diags
	}

	fmt.Print(1)
	if config.Flags.Dry || config.Flags.Prune {
		return nil
	}

	fmt.Print(2)
	lock, err := readLock(config.CWD)
	if err != nil {
		fmt.Print(21)
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "error reading lock",
			Detail:   err.Error(),
		}}
	}

	fmt.Print(3)
	lock.update(actions)
	err = lock.store(config.CWD)
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "error storing state",
			Detail:   err.Error(),
		}}
	}

	fmt.Print(4)
	return nil
}

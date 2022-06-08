package internal

import (
	"context"
	"encoding/json"
	"hash/crc64"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"bake/internal/lang"
	"bake/internal/lang/info"
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

	if !config.Flags.Dry && !config.Flags.Prune {
		err := store(config.CWD, actions)
		if err != nil {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary:  "error storing state",
				Detail:   err.Error(),
			}}
		}
	}

	return nil
}

const (
	StatePath         = ".bake"
	StateLockFilename = "lock.json"
)

type Lock struct {
	Version   string
	Timestamp time.Time
	Tasks     map[string]TaskHash
}

type TaskHash struct {
	// Creates keep a ref to the old filename in case it is renamed
	Creates string
	// hash the env and the command in case they change
	// Env     string TODO
	Command string
}

// TODO: a single execution of store isnt the whole picture so I need to merge
// the old state with the new one
func store(cwd string, actions []lang.Action) error {
	object := Lock{Version: info.Version, Timestamp: time.Now(), Tasks: map[string]TaskHash{}}
	for _, action := range actions {
		task, ok := action.(*lang.Task)
		if ok && task.ExitCode.Int64 == 0 && task.Creates != "" {
			checksum := crc64.Checksum([]byte(task.Command), crc64.MakeTable(crc64.ISO))
			object.Tasks[lang.PathString(task.GetPath())] = TaskHash{
				Creates: task.Creates,
				Command: strconv.FormatUint(checksum, 16),
			}
		}
	}

	statePath := filepath.Join(cwd, StatePath, StateLockFilename)
	err := os.MkdirAll(filepath.Dir(statePath), 0770)
	if err != nil {
		return err
	}

	file, err := os.Create(statePath)
	if err != nil {
		return err
	}

	// pretty print it for easier debugging
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(object)
}

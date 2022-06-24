package lang

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"bake/internal/concurrent"
	"bake/internal/lang/config"
	"bake/internal/lang/meta"
	"bake/internal/lang/schema"
	"bake/internal/lang/values"
	"bake/internal/paths"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/exp/maps"
)

type data struct {
	path     cty.Path
	metadata dataMetadata

	namedInstances   map[string]*dataInstance // for_each
	indexedInstances []*dataInstance          // count?
	singleInstance   *dataInstance            // plain task
}

type dataMetadata struct {
	Block hcl.Range // manual metadata
	// metadata from block
	Command hcl.Range
	Env     hcl.Range
}

func newData(raw addressBlock, eval *hcl.EvalContext) (Action, hcl.Diagnostics) {
	path := raw.GetPath()
	metadata := dataMetadata{Block: raw.Block.DefRange}
	diags := meta.DecodeRange(raw.Block.Body, eval, &metadata)
	if diags.HasErrors() {
		return nil, diags
	}

	forEachEntries, diags := schema.ForEachEntries(raw.Block, eval)
	if diags.HasErrors() {
		return nil, diags
	}

	if len(forEachEntries) == 0 {
		instance, diags := newDataInstance(path, metadata, raw.Block.Body, eval)
		if diags.HasErrors() {
			return nil, diags
		}

		return &data{
			path:           path,
			metadata:       metadata,
			singleInstance: instance,
		}, nil
	}

	instances := map[string]*dataInstance{}
	for key, value := range forEachEntries {
		ctx := eachContext(key, value, eval.NewChild())
		instance, diags := newDataInstance(path.IndexString(key), metadata, raw.Block.Body, ctx)
		if diags.HasErrors() {
			return nil, diags
		}

		instances[key] = instance
	}

	return &data{
		path:           path,
		metadata:       metadata,
		namedInstances: instances,
	}, nil
}

func (d data) GetPath() cty.Path {
	return d.path
}

func (d data) GetFilename() string {
	return d.metadata.Block.Filename
}

func (d data) CTY() cty.Value {
	if len(d.namedInstances) > 0 {
		m := map[string]cty.Value{}
		for k, instance := range d.namedInstances {
			m[k] = instance.CTY()
		}

		return cty.MapVal(m)
	}

	if len(d.indexedInstances) > 0 {
		m := make([]cty.Value, len(d.indexedInstances))
		for index, instance := range d.indexedInstances {
			m[index] = instance.CTY()
		}

		return cty.ListVal(m)
	}

	return d.singleInstance.CTY()
}

func (d data) Hash() []config.Hash {
	return nil
}

func (d *data) Apply(state *config.State) *sync.WaitGroup {
	if len(d.namedInstances) > 0 {
		return applyIndexed(maps.Values(d.namedInstances), state)
	}

	if len(d.indexedInstances) > 0 {
		return applyIndexed(d.indexedInstances, state)
	}

	return applySingle(d.singleInstance, state)
}

type dataInstance struct {
	path     cty.Path
	metadata dataMetadata

	Command  string            `hcl:"command,optional"`
	Env      map[string]string `hcl:"env,optional"`
	Remain   hcl.Body          `hcl:",remain"`
	StdOut   values.EventualString
	StdErr   values.EventualString
	ExitCode values.EventualInt64
}

func newDataInstance(path cty.Path, metadata dataMetadata, body hcl.Body, eval *hcl.EvalContext) (*dataInstance, hcl.Diagnostics) {
	data := &dataInstance{path: path, metadata: metadata}
	diags := gohcl.DecodeBody(body, eval, data)
	if diags.HasErrors() {
		return nil, diags
	}

	diags = schema.ValidateAttributes(data.Remain)
	if diags.HasErrors() {
		return nil, diags
	}

	// overwrite default env with custom values
	data.Env = concurrent.Merge(config.Env(), data.Env)
	return data, nil
}

func (d dataInstance) CTY() cty.Value {
	return values.StructToCty(d)
}

func (d *dataInstance) Apply(state *config.State) hcl.Diagnostics {
	if d.ExitCode.Valid { // apply data even on dry run
		return nil
	}

	log := NewLogger(d.path)
	log.Println(`refreshing ...`)
	// which shell should I use?
	terminal := "bash"
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		terminal = shell
	}

	// use shell with fail fast flags
	script := fmt.Sprintf(`
	set -euo pipefail

	%s`, d.Command)
	command := exec.CommandContext(state.Context, terminal, "-c", script)
	command.Env = config.EnvSlice(d.Env)
	// todo: should I allow configuring these?
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	start := time.Now()
	err := command.Run()
	end := time.Now()
	// store results
	d.StdOut = values.EventualString{
		String: strings.TrimSpace(stdout.String()),
		Valid:  true,
	}

	d.StdErr = values.EventualString{
		String: strings.TrimSpace(stderr.String()),
		Valid:  true,
	}

	d.ExitCode = values.EventualInt64{
		Int64: int64(command.ProcessState.ExitCode()),
		Valid: true,
	}

	detail := d.StdErr.String
	if detail == "" {
		detail = d.StdOut.String
	}

	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`"%s" command failed with: %s`, paths.String(d.path), command.ProcessState.String()),
			Detail:   detail,
			Subject:  &d.metadata.Command,
			Context:  &d.metadata.Block,
		}}
	}

	log.Println(`done in ` + end.Sub(start).String())
	return nil
}

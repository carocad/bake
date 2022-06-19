package lang

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"bake/internal/concurrent"
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"bake/internal/lang/values"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type data struct {
	addressBlock
	Command  string            `hcl:"command,optional"`
	Env      map[string]string `hcl:"env,optional"`
	StdOut   values.EventualString
	StdErr   values.EventualString
	ExitCode values.EventualInt64
}

func newData(raw addressBlock, ctx *hcl.EvalContext) (*data, hcl.Diagnostics) {
	data := &data{addressBlock: raw}
	diagnostics := gohcl.DecodeBody(raw.Block.Body, ctx, data)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	// overwrite default env with custom values
	data.Env = concurrent.Merge(config.Env(), data.Env)

	return data, nil
}

func (d data) CTY() cty.Value {
	value := values.StructToCty(d)
	m := value.AsValueMap()
	m[schema.NameLabel] = cty.StringVal(d.GetName())
	return cty.ObjectVal(m)
}

func (d data) Hash() *config.Hash {
	return nil
}

func (d *data) Apply(ctx context.Context, state *config.State) hcl.Diagnostics {
	if d.ExitCode.Valid { // apply data even on dry run
		return nil
	}

	log := NewLogger(d)
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
	command := exec.CommandContext(ctx, terminal, "-c", script)
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
			Summary:  fmt.Sprintf(`"%s" command failed with: %s`, AddressToString(d), command.ProcessState.String()),
			Detail:   detail,
			Subject:  schema.GetRangeFor(d.Block, schema.CommandAttr),
			Context:  d.Block.DefRange.Ptr(),
		}}
	}

	log.Println(`done in ` + end.Sub(start).String())
	return nil
}

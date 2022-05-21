package lang

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"bake/internal/values"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type Data struct {
	addressBlock
	Command  string `hcl:"command,optional"`
	StdOut   values.EventualString
	StdErr   values.EventualString
	ExitCode values.EventualInt64
}

func NewData(raw addressBlock, ctx *hcl.EvalContext) (*Data, hcl.Diagnostics) {
	data := &Data{addressBlock: raw}
	diagnostics := gohcl.DecodeBody(raw.block.Body, ctx, data)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return data, nil
}

func (d Data) CTY() cty.Value {
	value := values.StructToCty(d)
	m := value.AsValueMap()
	m[NameLabel] = cty.StringVal(d.GetName())
	return cty.ObjectVal(m)
}

func (d *Data) Apply() hcl.Diagnostics {
	if d.ExitCode.Valid {
		return nil
	}

	log.Println("refreshing " + PathString(d.GetPath()))

	terminal := "bash"
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		terminal = shell
	}

	script := fmt.Sprintf(`
	set -euo pipefail

	%s`, d.Command)
	command := exec.Command(terminal, "-c", script)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	d.StdOut = values.EventualString{
		String: strings.TrimSpace(stdout.String()),
		Valid:  true,
	}

	d.StdErr = values.EventualString{
		String: strings.TrimSpace(stderr.String()),
		Valid:  true,
	}

	// todo: keep a ref to command.ProcessState since it contains useful info
	// like process time, exit code, etc
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
			Summary:  fmt.Sprintf(`"%s" command failed with exit code %d`, PathString(d.GetPath()), d.ExitCode.Int64),
			Detail:   detail,
			Subject:  getCommandRange(d.block),
			Context:  d.block.DefRange.Ptr(),
		}}
	}

	return nil
}

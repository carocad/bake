package lang

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"bake/internal/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Data struct {
	addressBlock
	Description string   `hcl:"description,optional"`
	Command     string   `hcl:"command,optional"`
	Remain      hcl.Body `hcl:",remain"`
	StdOut      values.EventualString
	StdErr      values.EventualString
	ExitCode    values.EventualInt64
}

func (p Data) Apply() hcl.Diagnostics {
	return nil
}

func (p Data) CTY() cty.Value {
	value := values.StructToCty(p)
	m := value.AsValueMap()
	m[NameLabel] = cty.StringVal(p.name)
	return cty.ObjectVal(m)
}

func (d *Data) Refresh() hcl.Diagnostics {
	if d.ExitCode.Valid {
		return nil
	}

	log.Println("refreshing " + PathString(d.Path()))

	terminal := "bash"
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		terminal = shell
	}

	script := fmt.Sprintf(`
	set -euo pipefail

	%s`, d.Command)
	command := exec.Command(terminal, "-c", script)
	output, err := command.Output()
	d.StdOut = values.EventualString{
		String: strings.TrimSpace(string(output)),
		Valid:  true,
	}
	// todo: keep a ref to command.ProcessState since it contains useful info
	// like process time, exit code, etc
	d.ExitCode = values.EventualInt64{
		Int64: int64(command.ProcessState.ExitCode()),
		Valid: true,
	}

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			d.StdErr = values.EventualString{
				String: strings.TrimSpace(string(ee.Stderr)),
				Valid:  true,
			}
		}

		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("%s failed with %s", d.Command, err.Error()),
		}}
	}

	// log.Println(command.String(), *task.ExitCode, *task.StdOut, task.StdErr)
	return nil
}

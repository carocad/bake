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

type Task struct { // todo: what is really optional?
	addressBlock
	Description string   `hcl:"description,optional"`
	Command     string   `hcl:"command,optional"`
	Creates     string   `hcl:"creates,optional"`
	Sources     []string `hcl:"sources,optional"`
	Filename    string   `hcl:"filename,optional"`
	Remain      hcl.Body `hcl:",remain"`
	// todo: I dont actually need this since only data exposes it
	StdOut   values.EventualString
	StdErr   values.EventualString
	ExitCode values.EventualInt64
}

func (t Task) CTY() cty.Value {
	value := values.StructToCty(t)
	m := value.AsValueMap()
	m[NameLabel] = cty.StringVal(t.name)
	return cty.ObjectVal(m)
}

func (t Task) Apply() hcl.Diagnostics {
	if t.ExitCode.Valid {
		return nil
	}

	log.Println("executing " + PathString(t.Path()))

	terminal := "bash"
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		terminal = shell
	}

	script := fmt.Sprintf(`
	set -euo pipefail

	%s`, t.Command)
	command := exec.Command(terminal, "-c", script)
	output, err := command.Output()
	t.StdOut = values.EventualString{
		String: strings.TrimSpace(string(output)),
		Valid:  true,
	}
	// todo: keep a ref to command.ProcessState since it contains useful info
	// like process time, exit code, etc
	t.ExitCode = values.EventualInt64{
		Int64: int64(command.ProcessState.ExitCode()),
		Valid: true,
	}

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			t.StdErr = values.EventualString{
				String: strings.TrimSpace(string(ee.Stderr)),
				Valid:  true,
			}
		}

		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("%s failed with %s", t.Command, err.Error()),
		}}
	}

	// log.Println(command.String(), *task.ExitCode, *task.StdOut, task.StdErr)
	return nil
}

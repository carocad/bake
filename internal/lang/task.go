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
	"github.com/zclconf/go-cty/cty"
)

type Task struct { // todo: what is really optional?
	addressBlock
	Description string   `hcl:"description,optional"`
	Command     string   `hcl:"command,optional"`
	Creates     string   `hcl:"creates,optional"`
	Sources     []string `hcl:"sources,optional"`
	Remain      hcl.Body `hcl:",remain"`
	exitCode    values.EventualInt64
}

func (t Task) CTY() cty.Value {
	value := values.StructToCty(t)
	m := value.AsValueMap()
	m[NameLabel] = cty.StringVal(t.GetName())
	return cty.ObjectVal(m)
}

func (t *Task) Apply() hcl.Diagnostics {
	if t.exitCode.Valid {
		return nil
	}

	log.Println("executing " + PathString(t.GetPath()))

	terminal := "bash"
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		terminal = shell
	}

	script := fmt.Sprintf(`
	set -euo pipefail

	%s`, t.Command)
	command := exec.Command(terminal, "-c", script)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	// todo: keep a ref to command.ProcessState since it contains useful info
	// like process time, exit code, etc
	t.exitCode = values.EventualInt64{
		Int64: int64(command.ProcessState.ExitCode()),
		Valid: true,
	}

	detail := strings.TrimSpace(stderr.String())
	if detail == "" {
		detail = strings.TrimSpace(stdout.String())
	}

	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`"%s" task failed with exit code %d`, PathString(t.GetPath()), t.exitCode.Int64),
			Detail:   detail,
			Subject:  getCommandRange(t.block),
			Context:  t.block.DefRange.Ptr(),
		}}
	}

	return nil
}

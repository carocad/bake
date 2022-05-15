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
	m[NameLabel] = cty.StringVal(t.GetName())
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
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	t.StdOut = values.EventualString{
		String: strings.TrimSpace(stdout.String()),
		Valid:  true,
	}
	// todo: keep a ref to command.ProcessState since it contains useful info
	// like process time, exit code, etc
	t.ExitCode = values.EventualInt64{
		Int64: int64(command.ProcessState.ExitCode()),
		Valid: true,
	}

	t.StdErr = values.EventualString{
		String: stderr.String(),
		Valid:  true,
	}
	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf(`"%s" command failed with exit code %d`, PathString(t.Path()), t.ExitCode.Int64),
			Detail:   t.StdErr.String,
			Subject:  getCommandRange(t.block),
			Context:  t.block.DefRange.Ptr(),
		}}
	}

	return nil
}

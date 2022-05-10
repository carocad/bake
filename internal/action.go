package internal

import (
	"fmt"
	"os"
	"os/exec"

	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type EventualStatus int

const (
	Pending EventualStatus = iota
	Running
	Completed
)

type Task struct {
	Name        string
	Command     string
	Description string
	// todo: this might be quite big so better clean it up if not used by any other task
	// todo: make these pointers to allow cty.UnknownValue based on status
	StdOut   *string
	StdErr   *string
	ExitCode *int
	// internal only
	dependsOn []hcl.Traversal
	status    EventualStatus
}

func NewTask(name string, attrs hcl.Attributes, ctx *hcl.EvalContext) (*Task, hcl.Diagnostics) {
	command, diagnostics := lang.String(lang.CommandAttr, attrs, ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	description, diagnostics := lang.String(lang.DescriptionAttr, attrs, ctx)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	deps, diagnostics := dependsOn(attrs)
	if diagnostics.HasErrors() {
		return nil, diagnostics
	}

	return &Task{
		Name:        name,
		Command:     command,
		Description: description,
		dependsOn:   deps,
	}, nil
}

type Action interface {
	GetName() string // todo: do I need a "special" getname method?
	Dependencies() []hcl.Traversal
	Status() EventualStatus
	Run() hcl.Diagnostics
	// Settle forces evaluation of expressions that depend on other
	// actions
	// Settle() hcl.Diagnostics
	Addressable
}

type Addressable interface {
	Path() cty.Path
}

func (task Task) GetName() string {
	return task.Name
}

func (task Task) Dependencies() []hcl.Traversal {
	return task.dependsOn
}

func (task Task) Status() EventualStatus {
	return task.status
}

func (task *Task) Run() hcl.Diagnostics {
	task.status = Running
	defer func() { task.status = Completed }()

	terminal := "bash"
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		terminal = shell
	}

	script := fmt.Sprintf(`
	set -euo pipefail

	%s`, task.Command)
	command := exec.Command(terminal, "-c", script)
	output, err := command.Output()
	if output != nil {
		outStr := string(output)
		task.StdOut = &outStr
	}
	// todo: keep a ref to command.ProcessState since it contains useful info
	// like process time, exit code, etc
	code := command.ProcessState.ExitCode()
	task.ExitCode = &code
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			stderr := string(ee.Stderr)
			task.StdErr = &stderr
		}
	}

	if err != nil {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("%s failed with %s", task.Command, err.Error()),
		}}
	}

	// log.Println(command.String(), *task.ExitCode, *task.StdOut, task.StdErr)
	return nil
}

func dependsOn(attrs hcl.Attributes) ([]hcl.Traversal, hcl.Diagnostics) {
	diagnostics := make(hcl.Diagnostics, 0)
	for name, attr := range attrs {
		if name == "depends_on" {
			variables, diags := lang.TupleOfReferences(attr)
			if diags.HasErrors() {
				diagnostics = diagnostics.Extend(diags)
				continue
			}
			return variables, nil
		}
	}

	return nil, diagnostics
}

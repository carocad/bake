package internal

import (
	"fmt"
	"os"
	"os/exec"

	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
)

type EventualStatus int

const (
	Pending EventualStatus = iota
	Running
	Completed
)

type Phony struct {
	Task
}

type Target struct {
	Task
}

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

type Action interface {
	GetName() string
	Dependencies() []hcl.Traversal
	Status() EventualStatus
	Run() hcl.Diagnostics
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

func NewPhony(phony lang.PhonyConfig) (*Phony, hcl.Diagnostics) {
	task, diags := newTask(phony.Name, phony.Command, phony.Remain)
	if diags.HasErrors() {
		return nil, diags
	}

	return &Phony{*task}, nil
}

func NewTarget(target lang.TargetConfig) (*Target, hcl.Diagnostics) {
	task, diags := newTask(target.Name, target.Command, target.Remain)
	if diags.HasErrors() {
		return nil, diags
	}

	return &Target{*task}, nil
}

func newTask(name, command string, remain hcl.Body) (*Task, hcl.Diagnostics) {
	attrs, diags := remain.JustAttributes()
	if diags.HasErrors() {
		return nil, diags
	}

	dependencies, diags := dependsOn(attrs)
	if diags.HasErrors() {
		return nil, diags
	}

	return &Task{
		Name:      name,
		Command:   command,
		dependsOn: dependencies,
	}, nil
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

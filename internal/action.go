package internal

import (
	"fmt"
	"os"
	"os/exec"

	"bake/internal/lang"
	"bake/internal/lang/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/zclconf/go-cty/cty"
)

type EventualStatus int

const (
	Pending EventualStatus = iota
	Running
	Completed
)

type Action interface {
	GetName() string
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
	var command string
	diags := gohcl.DecodeExpression(attrs[lang.CommandAttr].Expr, ctx, &command)
	if diags.HasErrors() {
		return nil, diags
	}

	var description string
	if attr, ok := attrs[lang.DescriptionAttr]; ok {
		diags = gohcl.DecodeExpression(attr.Expr, ctx, &description)
		if diags.HasErrors() {
			return nil, diags
		}
	}

	deps, diags := dependsOn(attrs)
	if diags.HasErrors() {
		return nil, diags
	}

	return &Task{
		Name:        name,
		Command:     command,
		Description: description,
		dependsOn:   deps,
	}, nil
}

func (task Task) CTY() cty.Value {
	return cty.ObjectVal(values.CTY(task))
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

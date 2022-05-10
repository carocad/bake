package action

import (
	"fmt"
	"log"
	"os"
	"os/exec"

	"bake/internal/lang"
	"bake/internal/values"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
)

type Task struct {
	Name        string
	Command     values.EventualString
	commandExpr hcl.Expression
	Description string
	// todo: this might be quite big so better clean it up if not used by any other task
	// todo: make these pointers to allow cty.UnknownValue based on status
	StdOut   *string
	StdErr   *string
	ExitCode *int
	// internal only
	dependsOn []hcl.Traversal
}

func NewTask(name string, attrs hcl.Attributes, ctx *hcl.EvalContext) (*Task, hcl.Diagnostics) {
	var description string
	if attr, ok := attrs[lang.DescriptionAttr]; ok {
		diags := gohcl.DecodeExpression(attr.Expr, ctx, &description)
		if diags.HasErrors() {
			return nil, diags
		}
	}

	dependsOn := make([]hcl.Traversal, 0)
	if attr, ok := attrs[lang.DependsOnAttr]; ok {
		variables, diags := lang.TupleOfReferences(attr)
		if diags.HasErrors() {
			return nil, diags
		}

		dependsOn = append(dependsOn, variables...)
	}

	commandExpr := attrs[lang.CommandAttr].Expr
	dependsOn = append(dependsOn, commandExpr.Variables()...)

	return &Task{
		Name:        name,
		Command:     values.EventualString{Valid: false},
		commandExpr: commandExpr,
		Description: description,
		dependsOn:   dependsOn,
	}, nil
}

func (task Task) GetName() string {
	return task.Name
}

func (task Task) Dependencies() []hcl.Traversal {
	return task.dependsOn
}

func (task *Task) Preload(ctx *hcl.EvalContext) hcl.Diagnostics {
	if task.Command.Valid {
		return nil
	}

	var command string
	diags := gohcl.DecodeExpression(task.commandExpr, ctx, &command)
	if diags.HasErrors() {
		return diags
	}

	task.Command = values.EventualString{
		String: command,
		Valid:  true,
	}

	return nil
}

func (task *Task) Run() hcl.Diagnostics {
	log.Println("executing " + task.Name)

	terminal := "bash"
	shell, ok := os.LookupEnv("SHELL")
	if ok {
		terminal = shell
	}

	script := fmt.Sprintf(`
	set -euo pipefail

	%s`, task.Command.String)
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
			Summary:  fmt.Sprintf("%s failed with %s", task.Command.String, err.Error()),
		}}
	}

	// log.Println(command.String(), *task.ExitCode, *task.StdOut, task.StdErr)
	return nil
}

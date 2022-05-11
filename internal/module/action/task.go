package action

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

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
	StdOut   values.EventualString
	StdErr   values.EventualString
	ExitCode values.EventualInt64
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

func (task *Task) Plan(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics) {
	if task.Command.Valid {
		return nil, nil
	}

	var command string
	diags := gohcl.DecodeExpression(task.commandExpr, ctx, &command)
	if diags.HasErrors() {
		return nil, diags
	}

	task.Command = values.EventualString{
		String: command,
		Valid:  true,
	}

	return []Action{task}, nil
}

func (task *Task) Apply() hcl.Diagnostics {
	if task.ExitCode.Valid {
		return nil
	}

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
	task.StdOut = values.EventualString{
		String: strings.TrimSpace(string(output)),
		Valid:  true,
	}
	// todo: keep a ref to command.ProcessState since it contains useful info
	// like process time, exit code, etc
	task.ExitCode = values.EventualInt64{
		Int64: int64(command.ProcessState.ExitCode()),
		Valid: true,
	}

	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			task.StdErr = values.EventualString{
				String: strings.TrimSpace(string(ee.Stderr)),
				Valid:  true,
			}
		}

		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("%s failed with %s", task.Command.String, err.Error()),
		}}
	}

	// log.Println(command.String(), *task.ExitCode, *task.StdOut, task.StdErr)
	return nil
}

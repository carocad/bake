package internal

import (
	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
)

type Recipe struct {
	Phonies []Phony
	Targets []Target
}

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
	// Run() todo: run command and populate status, stdout, stderr and exitcode
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

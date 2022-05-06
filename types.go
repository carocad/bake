package main

import (
	"bake/lang"
	"github.com/hashicorp/hcl/v2"
)

type Recipe struct {
	Phonies []Phony
	Targets []Target
}

type Phony struct {
	Task
}

type Target struct {
	Task
}

type Task struct {
	Name      string
	Command   string
	DependsOn []hcl.Traversal
}

type Identifiable interface {
	GetName() string
	Dependencies() []hcl.Traversal
}

func (task Task) GetName() string {
	return task.Name
}

func (task Task) Dependencies() []hcl.Traversal {
	return task.DependsOn
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
		DependsOn: dependencies,
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

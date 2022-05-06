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

func NewPhony(phony lang.PhonyConfig) (Phony, hcl.Diagnostics) {
	task := Task{
		Name:    phony.ID,
		Command: phony.Command,
	}
	diagnostics := make(hcl.Diagnostics, 0)
	attrs, diags := phony.Remain.JustAttributes()
	if diags.HasErrors() {
		diagnostics = append(diagnostics, diags...)
	}

	for name, attr := range attrs {
		if name == "depends_on" {
			task.DependsOn = attr.Expr.Variables()
		}
	}

	return Phony{task}, diagnostics
}

func NewTarget(target lang.TargetConfig) (Target, hcl.Diagnostics) {
	task := Task{
		Name:    target.ID,
		Command: target.Command,
	}
	diagnostics := make(hcl.Diagnostics, 0)
	attrs, diags := target.Remain.JustAttributes()
	if diags.HasErrors() {
		diagnostics = append(diagnostics, diags...)
	}

	for name, attr := range attrs {
		if name == "depends_on" {
			task.DependsOn = attr.Expr.Variables()
		}
	}

	return Target{task}, diagnostics
}

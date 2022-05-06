package lang

import "github.com/hashicorp/hcl/v2"

type Recipe struct {
	Phonies []PhonyConfig  `hcl:"phony,block"`
	Targets []TargetConfig `hcl:"target,block"`
}

type PhonyConfig struct {
	ID      string   `hcl:"id,label"`
	Command string   `hcl:"command,optional"`
	Remain  hcl.Body `hcl:",remain"`
}

type TargetConfig struct {
	ID      string   `hcl:"id,label"`
	Command string   `hcl:"command,optional"`
	Remain  hcl.Body `hcl:",remain"`
	// DependsOn []hcl.Traversal `hcl:"depends_on,optional"`
}

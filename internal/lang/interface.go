package lang

import "github.com/hashicorp/hcl/v2"

type Recipe struct {
	Phonies []PhonyConfig  `hcl:"phony,block"`
	Targets []TargetConfig `hcl:"target,block"`
}

type PhonyConfig struct {
	Name        string   `hcl:"name,label"`
	Description string   `hcl:"description,optional"`
	Command     string   `hcl:"command,optional"` // todo: defer to later since it might depend on other tasks
	Remain      hcl.Body `hcl:",remain"`
}

type TargetConfig struct {
	Name        string   `hcl:"name,label"`
	Description string   `hcl:"description,optional"`
	Command     string   `hcl:"command,optional"`
	Remain      hcl.Body `hcl:",remain"`
}

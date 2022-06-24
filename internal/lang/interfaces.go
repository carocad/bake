package lang

import (
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"bake/internal/paths"
	"log"
	"os"

	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mitchellh/colorstring"
	"github.com/zclconf/go-cty/cty"
)

type CliCommand struct{ Name, Description string }

func GetPublicTasks(addrs []config.RawAddress) []CliCommand {
	commands := make([]CliCommand, 0)
	for _, addr := range addrs {
		if schema.IsKnownPrefix(addr.GetPath()) {
			continue
		}

		// can only be task block
		block, ok := addr.(addressBlock)
		if !ok {
			continue
		}

		attrs, diags := block.Block.Body.JustAttributes()
		if diags.HasErrors() {
			continue
		}

		attr, ok := attrs[schema.DescriptionAttr]
		if !ok {
			continue
		}

		var description string
		diags = gohcl.DecodeExpression(attr.Expr, nil, &description)
		if diags.HasErrors() {
			continue
		}

		commands = append(commands, CliCommand{paths.String(addr.GetPath()), description})
	}

	return commands
}

func NewLogger(path cty.Path) *log.Logger {
	prefix := colorstring.Color("[bold]" + paths.String(path))
	// todo: change stdout according to state
	return log.New(os.Stdout, prefix+": ", 0)
}

package lang

import (
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"bake/internal/lang/values"
	"bake/internal/paths"
	"context"
	"log"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/mitchellh/colorstring"
	"github.com/zclconf/go-cty/cty"
)

type Address interface {
	GetName() string
	GetPath() cty.Path
	GetFilename() string
}

type Action interface {
	Address
	values.Cty
	Apply(context.Context, *config.State) hcl.Diagnostics
	Hash() *config.Hash
}

type RawAddress interface {
	Address
	Dependencies() ([]hcl.Traversal, hcl.Diagnostics)
	Decode(ctx *hcl.EvalContext) ([]Action, hcl.Diagnostics)
}

func AddressToString[T Address](addr T) string {
	return paths.String(addr.GetPath())
}

type CliCommand struct{ Name, Description string }

func GetPublicTasks(addrs []RawAddress) []CliCommand {
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

func NewLogger(addr Address) *log.Logger {
	prefix := colorstring.Color("[bold]" + AddressToString(addr))
	// todo: change stdout according to state
	return log.New(os.Stdout, prefix+": ", 0)
}

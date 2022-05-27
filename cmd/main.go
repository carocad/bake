package main

import (
	"bake/internal"
	"bake/internal/lang"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/urfave/cli"
)

func main() {
	// where are we?
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	log, diags := do(cwd)
	if diags.HasErrors() {
		log.WriteDiagnostics(diags)
		os.Exit(2)
	}
}

func do(cwd string) (hcl.DiagnosticWriter, hcl.Diagnostics) {
	// create a parser
	parser := hclparse.NewParser()
	// logger for diagnostics
	log := hcl.NewDiagnosticTextWriter(os.Stdout, parser.Files(), 78, true)

	addrs, diags := internal.ReadRecipes(cwd, parser)
	if diags.HasErrors() {
		return log, diags
	}

	err := App(addrs).Run(os.Args)
	if err != nil {
		return log, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  err.Error(),
		}}
	}

	return log, nil

	/* TODO
	config := state.NewConfig(cwd)
	config.Task = "main" // TODO
	diags = internal.Do(config, addrs)
	if diags.HasErrors() {
		log.WriteDiagnostics(diags)
	}

	return nil

	*/
}

const (
	DryRun = "dry-run"
	List   = "list"
	Prune  = "prune"
	// Watch  = "watch" TODO
)

func App(addrs []lang.RawAddress) *cli.App {
	app := &cli.App{
		Name:  "bake",
		Usage: "Build task orchestration",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  List,
				Usage: "List all public tasks",
			},
			&cli.BoolFlag{
				Name:  Prune,
				Usage: "Remove all files created by the current recipes",
			},
		},
	}

	commands := make(cli.Commands, 0)
	for _, addr := range addrs {
		if lang.IsKnownPrefix(addr.GetPath()) {
			continue
		}

		// can only be task block
		desc := description(addr)
		if desc == "" {
			continue
		}

		cmd := cli.Command{
			Name:  addr.GetName(),
			Usage: desc,
			Action: func(c *cli.Context) error {
				fmt.Println("added task: ", c.Args().First())
				return nil
			},
		}
		commands = append(commands, cmd)
	}

	app.Commands = commands
	return app
}

func description(addr lang.RawAddress) string {
	block, ok := addr.(lang.AddressBlock)
	if !ok {
		return ""
	}

	attrs, diags := block.Block.Body.JustAttributes()
	if diags.HasErrors() {
		return ""
	}

	attr, ok := attrs[lang.DescripionAttr]
	if !ok {
		return ""
	}

	var description string
	diags = gohcl.DecodeExpression(attr.Expr, nil, &description)
	if diags.HasErrors() {
		return ""
	}

	return description
}

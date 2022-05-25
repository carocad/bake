package main

import (
	"bake/internal"
	"bake/internal/lang"
	"bake/internal/state"
	"fmt"
	"os"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/urfave/cli"
)

func main() {
	// create a parser
	parser := hclparse.NewParser()
	// logger for diagnostics
	log := hcl.NewDiagnosticTextWriter(os.Stdout, parser.Files(), 78, true)
	// where are we?
	cwd, err := os.Getwd()
	if err != nil {
		Fatal(log, hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  "couldn't get current working directory",
			Detail:   err.Error(),
		}})
	}

	addrs, diags := internal.ReadRecipes(cwd, parser)
	if diags.HasErrors() {
		Fatal(log, diags)
	}

	app := App(addrs)
	err = app.Run(os.Args)
	os.Exit(2)

	config := state.NewConfig(cwd)
	config.Task = "main" // TODO
	diags = internal.Do(config, addrs)
	if diags.HasErrors() {
		Fatal(log, diags)
	}
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

		cmd := cli.Command{
			Name:  addr.GetName(),
			Usage: addr.GetFilename(),
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

func Fatal(log hcl.DiagnosticWriter, diagnostics hcl.Diagnostics) {
	log.WriteDiagnostics(diagnostics)
	os.Exit(1)
}

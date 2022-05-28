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
	// where are we?
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	log, err := do(cwd)
	// success, stop early
	if err == nil {
		os.Exit(0)
	}

	// did we get a diagnostic?
	if diags, ok := err.(hcl.Diagnostics); ok {
		log.WriteDiagnostics(diags)
		os.Exit(2)
	}

	// random err
	fmt.Println(err)
	os.Exit(3)
}

func do(cwd string) (hcl.DiagnosticWriter, error) {
	// create a parser
	parser := hclparse.NewParser()
	// logger for diagnostics
	log := hcl.NewDiagnosticTextWriter(os.Stdout, parser.Files(), 78, true)

	addrs, diags := internal.ReadRecipes(cwd, parser)
	if diags.HasErrors() {
		return log, diags
	}

	err := App(cwd, log, addrs).Run(os.Args)
	if err != nil {
		return log, err
	}

	return log, nil
}

const (
	DryRun = "dry-run"
	Prune  = "prune"
	// Watch  = "watch" TODO
)

var foo = "2"

var (
	PruneFlag = cli.BoolFlag{
		Name:  Prune,
		Usage: "Remove all files created by the current recipes",
	}
	DryRunFlag = cli.BoolFlag{
		Name:  DryRun,
		Usage: "Don't actually run any recipe; just print them",
	}
)

func App(cwd string, log hcl.DiagnosticWriter, addrs []lang.RawAddress) *cli.App {
	app := &cli.App{
		Name:  "bake",
		Usage: "Build task orchestration",
		Flags: []cli.Flag{
			PruneFlag,
		},
	}

	tasks := lang.GetPublicTasks(addrs)
	for _, task := range tasks {
		task := task
		cmd := cli.Command{
			Name:  task.Name,
			Usage: task.Description,
			Flags: []cli.Flag{
				PruneFlag,
				DryRunFlag,
			},
			Action: func(c *cli.Context) error {
				config := state.NewConfig(cwd, task.Name)
				diags := internal.Do(config, addrs)
				if diags.HasErrors() {
					return diags
				}

				return nil
			},
		}
		app.Commands = append(app.Commands, cmd)
	}

	return app
}

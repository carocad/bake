package main

import (
	"bake/internal"
	"bake/internal/lang"
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

	err := App(cwd, addrs).Run(os.Args)
	if err != nil {
		return log, err
	}

	return log, nil
}

const (
	Dry   = "dry"
	Prune = "prune"
	Force = "force"
	// Watch  = "watch" TODO
)

var (
	PruneFlag = cli.BoolFlag{
		Name:  Prune,
		Usage: "Remove all files created by the recipes and its dependencies",
	}
	DryFlag = cli.BoolFlag{
		Name:  Dry,
		Usage: "Don't actually run any recipe; just print them",
	}
	ForceFlag = cli.BoolFlag{
		Name:  Force,
		Usage: "Force the current task to run even if nothing changed",
	}
)

func App(cwd string, addrs []lang.RawAddress) *cli.App {
	tasks := lang.GetPublicTasks(addrs)
	usage := `Build task orchestration. 
		
		NOTE: The "commands" below only contain tasks which have a description.`
	app := &cli.App{
		Name:  "bake",
		Usage: usage,
		Flags: []cli.Flag{
			PruneFlag,
		},
	}
	for _, task := range tasks {
		task := task
		cmd := cli.Command{
			Name:  task.Name,
			Usage: task.Description,
			Flags: []cli.Flag{
				PruneFlag,
				DryFlag,
				ForceFlag,
			},
			Action: func(c *cli.Context) error {
				state := lang.NewState(cwd, task.Name)
				state.Dry = c.Bool(Dry)
				state.Prune = c.Bool(Prune)
				state.Force = c.Bool(Force)

				diags := internal.Do(state, addrs)
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

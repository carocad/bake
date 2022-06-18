package main

import (
	"bake/internal"
	"bake/internal/lang"
	"bake/internal/lang/config"
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/urfave/cli"
)

func main() {
	// setup signal handler
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// do what the user asked for
	log, err := do(ctx)
	// success, stop early
	if err == nil {
		os.Exit(0)
	}

	// did we get a diagnostic?
	if diags, ok := err.(hcl.Diagnostics); ok {
		log.WriteDiagnostics(diags)
		os.Exit(2)
	}

	// unknown err
	fmt.Println(err)
	os.Exit(3)
}

func do(ctx context.Context) (hcl.DiagnosticWriter, error) {
	// create a parser
	parser := hclparse.NewParser()
	// logger for diagnostics
	log := hcl.NewDiagnosticTextWriter(os.Stdout, parser.Files(), 78, true)
	// keep track of flags and other config related vars
	state, err := config.NewState()
	if err != nil {
		return log, err
	}

	// read bake files in the cwd
	addrs, diags := internal.ReadRecipes(state.CWD, parser)
	if diags.HasErrors() {
		return log, diags
	}

	tasks := lang.GetPublicTasks(addrs)
	app := &cli.App{
		Name: "bake",
		Usage: `Build task orchestration. 
		
		NOTE: The "commands" below only contain tasks which have a description.`,
		Flags: []cli.Flag{
			PruneFlag, // TODO: implement this
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
				state.Flags, err = config.NewStateFlags(c.Bool(Dry), c.Bool(Prune), c.Bool(Force))
				if err != nil {
					return err
				}

				start := time.Now()
				diags := internal.Do(ctx, c.Command.Name, state, addrs)
				end := time.Now()
				fmt.Printf("\ndone in %s\n", end.Sub(start).String())
				if diags.HasErrors() {
					return diags
				}

				return nil
			},
		}
		app.Commands = append(app.Commands, cmd)
	}

	err = app.Run(os.Args)
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

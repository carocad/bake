package main

import (
	"bake/internal"
	"bake/internal/lang"
	"bake/internal/state"
	"errors"
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

	code := do(cwd)
	os.Exit(int(code))
}

type ExitCode int

// Exit codes
const (
	OK ExitCode = iota
	ErrPanic
	ErrReadingFiles
	ErrTaskCrash
)

func do(cwd string) ExitCode {
	// create a parser
	parser := hclparse.NewParser()
	// logger for diagnostics
	log := hcl.NewDiagnosticTextWriter(os.Stdout, parser.Files(), 78, true)

	addrs, diags := internal.ReadRecipes(cwd, parser)
	if diags.HasErrors() {
		log.WriteDiagnostics(diags)
		return ErrReadingFiles
	}

	err := App(cwd, log, addrs).Run(os.Args)
	if err != nil {
		return ErrTaskCrash
	}

	return OK
}

const (
	DryRun = "dry-run"
	List   = "list"
	Prune  = "prune"
	// Watch  = "watch" TODO
)

func App(cwd string, log hcl.DiagnosticWriter, addrs []lang.RawAddress) *cli.App {
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

	tasks := lang.GetPublicTasks(addrs)
	for _, task := range tasks {
		cmd := cli.Command{
			Name:  task.Name,
			Usage: task.Description,
			Action: func(c *cli.Context) error {
				config := state.NewConfig(cwd, task.Name)
				return run(config, log, addrs)
			},
		}
		app.Commands = append(app.Commands, cmd)
	}

	return app
}

func run(config *state.Config, log hcl.DiagnosticWriter, addrs []lang.RawAddress) error {
	diags := internal.Do(config, addrs)
	if diags.HasErrors() {
		log.WriteDiagnostics(diags)
		return errors.New(diags.Error())
	}

	return nil
}

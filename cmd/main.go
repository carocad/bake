package main

import (
	"bake/internal"
	"bake/internal/info"
	"bake/internal/lang"
	"bake/internal/lang/config"
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"sync"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/urfave/cli/v2"
)

func main() {
	defer panicHandler()

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
	app := &cli.App{
		Name:     "bake",
		Usage:    `Build task orchestration`,
		Compiled: time.Now(),
		Version:  info.Version,
		Commands: []*cli.Command{{
			Name:  "list",
			Usage: "lists all public tasks; those that have a description",
			Action: func(c *cli.Context) error {
				// keep track of flags and other config related vars
				state, err := config.NewState(ctx)
				if err != nil {
					return err
				}

				// read bake files in the cwd
				addrs, diags := internal.ReadRecipes(state.CWD, parser)
				if diags.HasErrors() {
					return diags
				}

				tasks := lang.GetPublicTasks(addrs)
				for _, task := range tasks {
					fmt.Printf("%s\t%s", task.Name, task.Description)
				}
				return nil
			},
		}, {
			Name:  "run",
			Usage: "runs the provided task from bake files",
			Flags: []cli.Flag{
				&DryFlag,
				&ForceFlag,
				&PruneFlag,
			},
			Action: func(c *cli.Context) error {
				task := c.Args().Get(0)
				if task == "" {
					return cli.ShowCommandHelp(c, c.Command.Name)
				}

				// keep track of flags and other config related vars
				state, err := config.NewState(ctx)
				if err != nil {
					return err
				}

				// read bake files in the cwd
				addrs, diags := internal.ReadRecipes(state.CWD, parser)
				if diags.HasErrors() {
					return diags
				}

				state.Flags, err = config.NewStateFlags(c.Bool(Dry), c.Bool(Prune), c.Bool(Force))
				if err != nil {
					return err
				}

				start := time.Now()
				diags = internal.Do(task, state, addrs)
				end := time.Now()
				fmt.Printf("\ndone in %s\n", end.Sub(start).String())
				if diags.HasErrors() {
					return diags
				}

				return nil
			},
		},
		},
	}

	err := app.Run(os.Args)
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

const panicOutput = `
!!!!!!!!!!!!!!!!!!!!!!!!!!! BAKE CRASH !!!!!!!!!!!!!!!!!!!!!!!!!!!!

Bake crashed! This is always indicative of a bug within Bake.
Please report the crash with Bake[1] so that we can fix this.

When reporting bugs, please include your bake version, the stack trace
shown below, and any additional information which may help replicate the issue.

[1]: https://github.com/carocad/bake/issues

!!!!!!!!!!!!!!!!!!!!!!!!!!! BAKE CRASH !!!!!!!!!!!!!!!!!!!!!!!!!!!!
`

// In case multiple goroutines panic concurrently, ensure only the first one
// recovered by PanicHandler starts printing.
var panicMutex sync.Mutex

// panicHandler is called to recover from an internal panic in bake, and
// augments the standard stack trace with a more user friendly error message.
// panicHandler must be called as a defered function, and must be the first
// defer called at the start of a new goroutine.
func panicHandler() {
	// Have all managed goroutines checkin here, and prevent them from exiting
	// if there's a panic in progress. While this can't lock the entire runtime
	// to block progress, we can prevent some cases where bake may return
	// early before the panic has been printed out.
	panicMutex.Lock()
	defer panicMutex.Unlock()

	recovered := recover()
	if recovered == nil {
		return
	}

	fmt.Fprint(os.Stderr, panicOutput)
	fmt.Fprint(os.Stderr, recovered, "\n")

	debug.PrintStack()

	// An exit code of 11 keeps us out of the way of the detailed exitcodes
	// from plan, and also happens to be the same code as SIGSEGV which is
	// roughly the same type of condition that causes most panics.
	os.Exit(11)
}

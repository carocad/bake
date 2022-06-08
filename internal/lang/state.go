package lang

import (
	"bake/internal/concurrent"
	"bake/internal/lang/info"
	"encoding/json"
	"hash/crc64"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2"
	"github.com/mitchellh/colorstring"
	"github.com/zclconf/go-cty/cty"
)

type State struct {
	CWD string
	// Context     context.Context TODO
	Env         map[string]string
	Args        []string
	Dry         bool
	Prune       bool
	Force       bool
	Parallelism uint8
	Task        string
}

const DefaultParallelism = 4

func NewState() (*State, error) {
	// where are we?
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// organize out env vars
	env := map[string]string{}
	for _, keyVal := range os.Environ() {
		parts := strings.SplitN(keyVal, "=", 2)
		key, val := parts[0], parts[1]
		env[key] = val
	}

	return &State{
		CWD:         cwd,
		Env:         env,
		Args:        os.Args,
		Parallelism: DefaultParallelism,
	}, nil
}

func (state State) Context(addr RawAddress, actions []Action) *hcl.EvalContext {
	variables := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(state.CWD),
			"module":  cty.StringVal(filepath.Join(state.CWD, filepath.Dir(addr.GetFilename()))),
			"current": cty.StringVal(filepath.Join(state.CWD, addr.GetFilename())),
		}),
	}

	data := map[string]cty.Value{}
	local := map[string]cty.Value{}
	task := map[string]cty.Value{}
	for _, act := range actions {
		name := act.GetName()
		path := act.GetPath()
		value := act.CTY()
		switch {
		case path.HasPrefix(DataPrefix):
			data[name] = value
		case path.HasPrefix(LocalPrefix):
			local[name] = value
		default:
			task[name] = value
		}
	}

	variables[DataLabel] = cty.ObjectVal(data)
	variables[LocalScope] = cty.ObjectVal(local)
	variables[TaskLabel] = cty.ObjectVal(task)
	// allow tasks to be referred without a prefix
	concurrent.Merge(variables, task)
	return &hcl.EvalContext{
		Variables: variables,
		Functions: Functions(),
	}
}

func (state State) NewLogger(addr Address) *log.Logger {
	prefix := colorstring.Color("[bold]" + PathString(addr.GetPath()))
	// todo: change stdout according to state
	return log.New(os.Stdout, prefix+": ", 0)
}

const StatePath = ".bake"

type Lock struct {
	Version   string
	Timestamp time.Time
	Tasks     map[string]TaskHash
}

type TaskHash struct {
	// Creates keep a ref to the old filename in case it is renamed
	Creates string
	// hash the env and the command in case they change
	// Env     string TODO
	Command string
}

// TODO: a single execution of Store isnt the whole picture so I need to merge
// the old state with the new one
func (state State) Store(actions []Action) error {
	object := Lock{Version: info.Version, Timestamp: time.Now(), Tasks: map[string]TaskHash{}}
	for _, action := range actions {
		task, ok := action.(*Task)
		if ok && task.ExitCode.Int64 == 0 && task.Creates != "" {
			checksum := crc64.Checksum([]byte(task.Command), crc64.MakeTable(crc64.ISO))
			object.Tasks[PathString(task.GetPath())] = TaskHash{
				Creates: task.Creates,
				Command: strconv.FormatUint(checksum, 16),
			}
		}
	}

	statePath := filepath.Join(state.CWD, StatePath, "lock.json")
	err := os.MkdirAll(filepath.Dir(statePath), 0770)
	if err != nil {
		return err
	}

	file, err := os.Create(statePath)
	if err != nil {
		return err
	}

	// pretty print it for easier debugging
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(object)
}

package module

import (
	"bake/internal/concurrent"
	"bake/internal/lang"
	"bake/internal/lang/config"
	"bake/internal/lang/schema"
	"bake/internal/module/topo"
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
	"golang.org/x/sync/errgroup"
)

// example from make target --dry-run --debug
/* TARGET
% make server/cmd/main.bin --dry-run --debug
GNU Make 4.3
Built for x86_64-apple-darwin20.1.0
Copyright (C) 1988-2020 Free Software Foundation, Inc.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
Reading makefiles...
Updating makefiles....
Updating goal targets....
 Prerequisite 'go.mod' is newer than target 'server/cmd/main.bin'.
 Prerequisite 'internal/strings.go' is newer than target 'server/cmd/main.bin'.
 Prerequisite 'server/system.go' is newer than target 'server/cmd/main.bin'.
 Prerequisite 'server/hardware/artifactory/artifactory_release_sets.go' is newer than target 'server/cmd/main.bin'.
 Prerequisite 'server/hardware/artifactory/artifactory_release_set_test.go' is newer than target 'server/cmd/main.bin'.
 Prerequisite 'server/hardware/mysql/artifactory_set.go' is newer than target 'server/cmd/main.bin'.
 Prerequisite 'cli/cmd/main.go' is newer than target 'server/cmd/main.bin'.
 Prerequisite 'cli/system.go' is newer than target 'server/cmd/main.bin'.
 Prerequisite 'cli/launch.go' is newer than target 'server/cmd/main.bin'.
 Prerequisite 'cli/launch_test.go' is newer than target 'server/cmd/main.bin'.
Must remake target 'server/cmd/main.bin'.
go test -c -v -tags=testIntegration -coverpkg=./... -o server/cmd/main.bin ./server/cmd/
Successfully remade target file 'server/cmd/main.bin'.
*/

/* PHONY
% make dev --dry-run --debug
GNU Make 4.3
Built for x86_64-apple-darwin20.1.0
Copyright (C) 1988-2020 Free Software Foundation, Inc.
License GPLv3+: GNU GPL version 3 or later <http://gnu.org/licenses/gpl.html>
This is free software: you are free to change and redistribute it.
There is NO WARRANTY, to the extent permitted by law.
Reading makefiles...
Updating makefiles....
Updating goal targets....
 File 'dev' does not exist.
   File 'compose' does not exist.
     File 'docker-base' does not exist.
    Must remake target 'docker-base'.
docker build -t devops/launch-assist/base:$(git describe --tags --abbrev=0) -t devops/launch-assist/base:$(git rev-parse --abbrev-ref HEAD | tr A-Z/ a-z-)-0 -t devops/launch-assist/base:latest --cache-from=devops/launch-assist/base --build-arg BUILDKIT_INLINE_CACHE=1 --build-arg GOPROXY='https://proxy.golang.org,direct' --build-arg NPM_USER --build-arg NPM_PASS -f Dockerfile .
    Successfully remade target file 'docker-base'.
  Must remake target 'compose'.
docker-compose -f docker-compose.yaml up --build --remove-orphans
  Successfully remade target file 'compose'.
Must remake target 'dev'.
Successfully remade target file 'dev'.
*/

// Coordinator executes tasks in parallel respecting the dependencies between each task
type Coordinator struct {
	// use a bounded go routine pool for the execution of task commands
	pool    *errgroup.Group
	waiting *concurrent.Map[lang.Address, *sync.WaitGroup]
	actions *concurrent.Slice[lang.Action]
	state   config.State
}

func NewCoordinator(ctx context.Context, eval config.State) Coordinator {
	// todo: do I need to return the context?
	bounded, _ := errgroup.WithContext(ctx)
	bounded.SetLimit(int(eval.Parallelism))
	return Coordinator{
		pool:    bounded,
		waiting: concurrent.NewMapBy[lang.Address, *sync.WaitGroup](lang.AddressToString[lang.Address]),
		actions: concurrent.NewSlice[lang.Action](),
		state:   eval,
	}
}

// Do task with id on a separate go routine after its dependencies are done. All dependencies MUST
// have a previously registered task, otherwise the entire task coordinator
// is stopped and an error is returned
func (coordinator *Coordinator) Do(task lang.RawAddress, addresses []lang.RawAddress) ([]lang.Action, hcl.Diagnostics) {
	allDependencies, diags := topo.AllDependencies(task, addresses)
	if diags.HasErrors() {
		return nil, diags
	}

	taskDependencies, _ := allDependencies.Get(task)
	for _, address := range taskDependencies {
		// initialize this dependency wait group so that other goroutines can wait for it
		promise := &sync.WaitGroup{}
		coordinator.waiting.Put(address, promise)

		// get the dependencies of this task dependency
		addressDependencies, _ := allDependencies.Get(address)
		// wait for all routines to finish so that we get all actions
		// we need to remove the last element since it is the address itself
		diags := coordinator.waitFor(addressDependencies[:len(addressDependencies)-1])
		if diags.HasErrors() {
			return nil, diags
		}

		// evalContext := coordinator.state.Context(address, coordinator.actions.Items())
		evalContext := coordinator.context(address, coordinator.actions.Items())
		actions, diags := address.Decode(evalContext)
		if diags.HasErrors() {
			return nil, diags
		}

		coordinator.actions.Extend(actions)
		for _, action := range actions {
			promise.Add(1)
			// keep a reference to the original value due to closure and goroutine
			// https://golang.org/doc/faq#closures_and_goroutines
			action := action
			// use the bounded routine pool to avoid overloading the OS with possibly
			// CPU heavy tasks
			coordinator.pool.Go(func() error {
				defer promise.Done()

				diags := action.Apply(coordinator.state)
				// todo: display the time it took to apply the action
				if diags.HasErrors() {
					return diags
				}

				return nil
			})
		}
	}

	err := coordinator.pool.Wait()
	if diags, ok := err.(hcl.Diagnostics); ok {
		return nil, diags
	}

	return coordinator.actions.Items(), nil
}

func (coordinator *Coordinator) waitFor(dependencies []lang.RawAddress) hcl.Diagnostics {
	for _, dep := range dependencies {
		waiter, ok := coordinator.waiting.Get(dep)
		if !ok {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary: fmt.Sprintf("missing task %s",
					lang.AddressToString(dep)),
			}}
		}

		waiter.Wait()
	}

	return nil
}

func (coordinator *Coordinator) context(addr lang.Address, actions []lang.Action) *hcl.EvalContext {
	parent := coordinator.state.Context()
	child := parent.NewChild()

	cwd := coordinator.state.CWD
	variables := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(cwd),
			"module":  cty.StringVal(filepath.Join(cwd, filepath.Dir(addr.GetFilename()))),
			"current": cty.StringVal(filepath.Join(cwd, addr.GetFilename())),
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
		case path.HasPrefix(schema.DataPrefix):
			data[name] = value
		case path.HasPrefix(schema.LocalPrefix):
			local[name] = value
		default:
			task[name] = value
		}
	}

	variables[schema.DataLabel] = cty.ObjectVal(data)
	variables[schema.LocalScope] = cty.ObjectVal(local)
	variables[schema.TaskLabel] = cty.ObjectVal(task)
	// allow tasks to be referred without a prefix
	concurrent.Merge(variables, task)

	child.Variables = variables
	return child
}

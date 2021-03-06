package module

import (
	"bake/internal/concurrent"
	"bake/internal/lang/config"
	"bake/internal/module/topo"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
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
	waiting *concurrent.Map[config.Address, *sync.WaitGroup]
	actions *concurrent.Slice[config.Action]
}

func NewCoordinator() Coordinator {
	return Coordinator{
		waiting: concurrent.NewMapBy[config.Address, *sync.WaitGroup](config.AddressToString[config.Address]),
		actions: concurrent.NewSlice[config.Action](),
	}
}

// Do task with id on a separate go routine after its dependencies are done. All dependencies MUST
// have a previously registered task, otherwise the entire task coordinator
// is stopped and an error is returned
func (coordinator *Coordinator) Do(state *config.State, task config.RawAddress, addresses []config.RawAddress) ([]config.Action, hcl.Diagnostics) {
	allDependencies, diags := topo.AllDependencies(task, addresses)
	if diags.HasErrors() {
		return nil, diags
	}

	taskDependencies := allDependencies[config.AddressToString(task)]
	for _, address := range taskDependencies {
		// get the dependencies of this task dependency
		addressDependencies := allDependencies[config.AddressToString(address)]
		// wait for all routines to finish so that we get all actions
		// we need to remove the last element since it is the address itself
		diags := coordinator.waitFor(addressDependencies[:len(addressDependencies)-1])
		if diags.HasErrors() {
			return nil, diags
		}

		evalContext := state.EvalContext()
		evalContext.Variables = concurrent.Merge(
			pathEvalContext(state, address),
			config.Actions(coordinator.actions.Items()).EvalContext(),
		)
		action, diags := address.Decode(evalContext)
		if diags.HasErrors() {
			return nil, diags
		}

		wait := action.Apply(state)
		// initialize this dependency wait group so that other goroutines can wait for it
		coordinator.waiting.Put(address, wait)
		coordinator.actions.Append(action)
	}

	err := state.Group.Wait()
	if diags, ok := err.(hcl.Diagnostics); ok {
		return nil, diags
	}

	return coordinator.actions.Items(), nil
}

func (coordinator *Coordinator) waitFor(dependencies []config.RawAddress) hcl.Diagnostics {
	for _, dep := range dependencies {
		group, ok := coordinator.waiting.Get(dep)
		if !ok {
			return hcl.Diagnostics{{
				Severity: hcl.DiagError,
				Summary: fmt.Sprintf("missing task %s",
					config.AddressToString(dep)),
			}}
		}

		if group == nil {
			continue
		}

		group.Wait()
	}

	return nil
}

func pathEvalContext(state *config.State, addr config.Address) map[string]cty.Value {
	cwd := state.CWD
	return map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(cwd),
			"module":  cty.StringVal(filepath.Join(cwd, filepath.Dir(addr.GetFilename()))),
			"current": cty.StringVal(filepath.Join(cwd, addr.GetFilename())),
		}),
	}
}

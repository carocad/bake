package contextualize

import (
	"log"
	"path/filepath"

	"bake/internal/lang"
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Semaphore struct {
	fileAddrs map[string][]lang.RawAddress
	cwd       string
	actions   []lang.Action
	promises  map[string]promise
}

type promise struct {
	lang.RawAddress
	channel chan bool
}

func NewSemaphore(cwd string, addrs map[string][]lang.RawAddress) *Semaphore {
	return &Semaphore{
		fileAddrs: addrs,
		cwd:       cwd,
		actions:   make([]lang.Action, 0),
		promises:  map[string]promise{},
	}
}

func (sem *Semaphore) Context(addr lang.RawAddress) *hcl.EvalContext {
	addrFile := ""
	for filename, addresses := range sem.fileAddrs {
		for _, address := range addresses {
			if address.Path().Equals(addr.Path()) {
				addrFile = filename
				break
			}
		}
	}

	if addrFile == "" {
		panic("couldn't find address on read files, please notify bake developers")
	}

	variables := map[string]cty.Value{
		"path": cty.ObjectVal(map[string]cty.Value{
			"root":    cty.StringVal(sem.cwd),
			"module":  cty.StringVal(filepath.Join(sem.cwd, filepath.Dir(addrFile))),
			"current": cty.StringVal(filepath.Join(sem.cwd, addrFile)),
		}),
	}

	data := map[string]cty.Value{}
	local := map[string]cty.Value{}
	for _, act := range sem.actions {
		name := act.GetName()
		path := act.Path()
		value := act.CTY()
		switch {
		case path.HasPrefix(lang.DataPrefix):
			data[name] = value
		case path.HasPrefix(lang.LocalPrefix):
			local[name] = value
		default:
			// only targets for now !!
			variables[name] = value
		}
	}

	variables[lang.DataLabel] = cty.ObjectVal(data)
	variables[lang.LocalScope] = cty.ObjectVal(local)
	return &hcl.EvalContext{
		Variables: variables,
		Functions: lang.Functions(),
	}
}

func (sem *Semaphore) WaitFor(addr lang.RawAddress) <-chan bool {
	channel := make(chan bool, 1)
	if sem.isReady(addr) {
		channel <- true
	} else {
		sem.promises[lang.PathString(addr.Path())] = promise{
			RawAddress: addr,
			channel:    channel,
		}
	}

	return channel
}

func (sem *Semaphore) isReady(addr lang.RawAddress) bool {
	ctx := sem.Context(addr)
	dependencies, diagnostics := addr.Dependencies()
	if diagnostics.HasErrors() {
		return false
	}

	for _, dependency := range dependencies {
		value, diagnostics := dependency.TraverseAbs(ctx)
		if diagnostics.HasErrors() {
			return false
		}

		if !value.IsWhollyKnown() {
			return false
		}
	}

	return true
}

func (sem *Semaphore) Extend(actions []lang.Action) {
	sem.actions = append(sem.actions, actions...)
	sem.deliver()
}

func (sem *Semaphore) Append(action lang.Action) {
	sem.actions = append(sem.actions, action)
	sem.deliver()
}

// deliver check if a call to Append or Extend unblocks an Action
func (sem *Semaphore) deliver() {
	for path, promise := range sem.promises {
		if sem.isReady(promise.RawAddress) {
			promise.channel <- true
			delete(sem.promises, path)
			log.Println("delivered to " + path)
		}
	}
}

func (sem *Semaphore) GetActions() []lang.Action {
	return sem.actions
}

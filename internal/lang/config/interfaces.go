package config

import (
	"bake/internal/lang/values"
	"bake/internal/paths"
	"sync"

	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

type Address interface {
	GetPath() cty.Path
	GetFilename() string
}

type Action interface {
	Address
	values.Cty
	RuntimeAction
	Hasher
}

type RuntimeAction interface {
	Apply(state *State) *sync.WaitGroup
}

type RuntimeInstance interface {
	Apply(state *State) hcl.Diagnostics
}

type RawAddress interface {
	Address
	Dependencies() ([]hcl.Traversal, hcl.Diagnostics)
	Decode(ctx *hcl.EvalContext) (Action, hcl.Diagnostics)
}

func AddressToString[T Address](addr T) string {
	return paths.String(addr.GetPath())
}

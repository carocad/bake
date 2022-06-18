package schema

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/zclconf/go-cty/cty"
)

// labels
const (
	TaskLabel   = "task"
	DataLabel   = "data"
	LocalsLabel = "locals"
	NameLabel   = "name"
)

const (
	// LocalScope only for locals since the scope != label
	LocalScope = "local"
	// PathScope is automatically injected
	PathScope = "path"
	// EachScope is automatically injected on resources with for_each meta argument
	EachScope = "each"
)

// attributes
const (
	DependsOnAttr   = "depends_on"
	CommandAttr     = "command"
	CreatesAttr     = "creates"
	SourcesAttr     = "sources"
	DescriptionAttr = "description"
	ForEachAttr     = "for_each"
)

var (
	TaskPrefix  = cty.GetAttrPath(TaskLabel)
	DataPrefix  = cty.GetAttrPath(DataLabel)
	LocalPrefix = cty.GetAttrPath(LocalScope)
	PathPrefix  = cty.GetAttrPath(PathScope)
	EachPrefix  = cty.GetAttrPath(EachScope)
	// KnownPrefixes are the prefixes assigned to anything that is NOT a task
	KnownPrefixes = cty.NewPathSet(DataPrefix, LocalPrefix, PathPrefix, EachPrefix)
	// IgnorePrefixes are those automatically injected by bake instead of defined by
	// user input
	IgnorePrefixes = cty.NewPathSet(PathPrefix, EachPrefix)
)

func IsKnownPrefix(path cty.Path) bool {
	for _, prefix := range KnownPrefixes.List() {
		if path.HasPrefix(prefix) {
			return true
		}
	}

	return false
}

func FileSchema() *hcl.BodySchema {
	return &hcl.BodySchema{
		Attributes: nil,
		Blocks: []hcl.BlockHeaderSchema{{
			Type:       TaskLabel,
			LabelNames: []string{NameLabel},
		}, {
			Type:       DataLabel,
			LabelNames: []string{NameLabel},
		}, {
			Type:       LocalsLabel,
			LabelNames: []string{},
		}},
	}
}

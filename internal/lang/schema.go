package lang

import "github.com/hashicorp/hcl/v2"

// labels
const (
	TargetLabel = "target"
	PhonyLabel  = "phony"
	NameLabel   = "name"
)

// attributes
const (
	DescriptionAttr = "description"
	CommandAttr     = "command"
	FilenameAttr    = "filename"
	SourcesAttr     = "sources"
	DependsOnAttr   = "depends_on"
	ForEachAttr     = "for_each"
)

func RecipeSchema() *hcl.BodySchema {
	return &hcl.BodySchema{
		Attributes: nil,
		Blocks: []hcl.BlockHeaderSchema{{
			Type:       TargetLabel,
			LabelNames: []string{NameLabel},
		}, {
			Type:       PhonyLabel,
			LabelNames: []string{NameLabel},
		}},
	}
}

func TargetSchema() *hcl.BodySchema {
	return &hcl.BodySchema{Attributes: []hcl.AttributeSchema{{
		Name:     DescriptionAttr,
		Required: false,
	}, {
		Name:     CommandAttr,
		Required: true,
	}, {
		Name:     FilenameAttr,
		Required: true,
	}, {
		Name:     SourcesAttr,
		Required: true,
	}, {
		Name:     DependsOnAttr,
		Required: false,
	}}}
}

func PhonySchema() *hcl.BodySchema {
	return &hcl.BodySchema{Attributes: []hcl.AttributeSchema{{
		Name:     DescriptionAttr,
		Required: false,
	}, {
		Name:     CommandAttr,
		Required: true,
	}, {
		Name:     DependsOnAttr,
		Required: false,
	}, {
		Name:     ForEachAttr,
		Required: false,
	}}}
}

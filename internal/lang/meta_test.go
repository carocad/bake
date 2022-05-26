package lang

import (
	"reflect"
	"testing"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestDecodeRange(t *testing.T) {
	// arrange
	srange := hcl.Range{
		Filename: "world.go",
		Start:    hcl.InitialPos,
		End:      hcl.Pos{Byte: 100, Line: 1, Column: 100},
	}
	body := hclsyntax.Body{
		Attributes: hclsyntax.Attributes{
			"hello_world": &hclsyntax.Attribute{
				Name:     "hello_world",
				SrcRange: srange,
			},
		},
	}

	value := struct {
		HelloWorld hcl.Range
	}{}

	// act
	diags := DecodeRange(&body, nil, &value)
	// assert
	if diags.HasErrors() {
		t.Fatal(diags)
	}

	if !reflect.DeepEqual(value.HelloWorld, srange) {
		t.Error("expected decoded range to be equal to input one")
		t.Logf("%#v", value.HelloWorld)
	}

}

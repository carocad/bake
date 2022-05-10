package values

import (
	"reflect"

	"github.com/zclconf/go-cty/cty"
)

type Eventual interface {
	CTY() cty.Value
}

var eventualType = reflect.TypeOf((*Eventual)(nil)).Elem()

type EventualString struct {
	String string
	Valid  bool // Valid is true if String is known
}

func (this EventualString) CTY() cty.Value {
	if this.Valid {
		return cty.StringVal(this.String)
	}

	return cty.UnknownVal(cty.String)
}

type EventualInt64 struct {
	Int64 int64
	Valid bool // Valid is true if Int64 is known
}

func (this EventualInt64) CTY() cty.Value {
	if this.Valid {
		return cty.NumberIntVal(this.Int64)
	}

	return cty.UnknownVal(cty.Number)
}

package values

import (
	"github.com/zclconf/go-cty/cty"
)

// Cty allows structs to customize the way they are
// converted to cty.Value
type Cty interface {
	CTY() cty.Value
}

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

type EventualStringSlice struct {
	Slice []string
	Valid bool // Valid is true if String is known
}

func (this EventualStringSlice) CTY() cty.Value {
	if this.Valid {
		result := make([]cty.Value, 0)
		for _, s := range this.Slice {
			result = append(result, cty.StringVal(s))
		}
		return cty.ListVal(result)
	}

	return cty.UnknownVal(cty.List(cty.String))
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

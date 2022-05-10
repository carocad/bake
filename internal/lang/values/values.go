package values

import (
	"reflect"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

// check json.typeFields for inspiration of reflect logic

// CTY converts go lang structs into cty maps automatically.
// By convention nil pointers are represented as cty.UnknownVal
// and once known the pointer should be set to an appropriate value
func CTY(instance interface{}) map[string]cty.Value {
	result := map[string]cty.Value{}
	val := reflect.Indirect(reflect.ValueOf(instance))
	for index := 0; index < val.NumField(); index++ {
		field := val.Type().Field(index)
		fieldValue := val.Field(index)
		// Ignore unexported types.
		if !field.IsExported() {
			continue
		}

		name := ToSnakeCase(field.Name)
		if reflect.PointerTo(field.Type).Implements(eventualType) {
			m, ok := fieldValue.Interface().(Eventual)
			if !ok {
				panic("value MUST implement Eventual interface")
			}

			result[name] = m.CTY()
			continue
		}

		item := fieldValue.Interface()
		impliedType, err := gocty.ImpliedType(item)
		if err != nil {
			panic(err) // should never be reached -> implies a 🐞 in the code
		}
		value, err := gocty.ToCtyValue(fieldValue.Interface(), impliedType)
		result[name] = value
	}

	return result
}

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
func CTY(instance interface{}) cty.Value {
	val := reflect.Indirect(reflect.ValueOf(instance))
	if reflect.PointerTo(val.Type()).Implements(eventualType) {
		m, ok := instance.(Eventual)
		if !ok {
			panic("value MUST implement Eventual interface")
		}

		return m.CTY()
	}

	result := map[string]cty.Value{}
	for index := 0; index < val.NumField(); index++ {
		field := val.Type().Field(index)
		fieldValue := val.Field(index)
		// Ignore unexported types.
		if !field.IsExported() {
			continue
		}

		name := ToSnakeCase(field.Name)
		fieldInterface := fieldValue.Interface()
		if reflect.PointerTo(field.Type).Implements(eventualType) {
			m, ok := fieldInterface.(Eventual)
			if !ok {
				panic("value MUST implement Eventual interface")
			}

			result[name] = m.CTY()
			continue
		}

		if field.Type.Kind() == reflect.Struct {
			m := CTY(fieldInterface)
			for k, v := range m.AsValueMap() {
				result[k] = v
			}
			continue
		}

		impliedType, err := gocty.ImpliedType(fieldInterface)
		if err != nil {
			panic(err) // should never be reached -> implies a 🐞 in the code
		}
		value, err := gocty.ToCtyValue(fieldInterface, impliedType)
		result[name] = value
	}

	return cty.ObjectVal(result)
}

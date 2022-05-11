package values

import (
	"fmt"
	"reflect"

	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/gocty"
)

// check json.typeFields for inspiration of reflect logic

func StructToCty(instance interface{}) cty.Value {
	val := reflect.Indirect(reflect.ValueOf(instance))
	result := map[string]cty.Value{}
	for index := 0; index < val.NumField(); index++ {
		field := val.Type().Field(index)
		fieldValue := val.Field(index)
		// Ignore unexported types.
		if !field.IsExported() {
			continue
		}

		fieldInterface := fieldValue.Interface()
		name := ToSnakeCase(field.Name)
		if v, ok := fieldInterface.(Cty); ok {
			result[name] = v.CTY()
			continue
		}

		// handle primitives conversions
		impliedType, err := gocty.ImpliedType(fieldInterface)
		if err != nil { // should never be reached -> implies a 🐞 in the code
			panic(fmt.Sprintf("couldn't find implied type: %s", err))
		}

		value, err := gocty.ToCtyValue(fieldInterface, impliedType)
		if err != nil { // should never be reached -> implies a 🐞 in the code
			panic(fmt.Sprintf("couldn't convert instance type: %s", err))
		}

		result[name] = value
	}

	return cty.ObjectVal(result)
}

package values

import (
	"fmt"
	"reflect"

	"github.com/zclconf/go-cty/cty"
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
		// Ignore unexported types.
		if !field.IsExported() {
			continue
		}

		name := ToSnakeCase(field.Name)
		switch field.Type.Kind() {
		case reflect.Struct:
			if reflect.PointerTo(field.Type).Implements(eventualType) {
				m, ok := val.Field(index).Interface().(Eventual)
				if !ok {
					panic("value MUST implement Eventual interface")
				}

				result[name] = m.CTY()
			} else {
				inner := CTY(val.Field(index).Interface())
				for key, value := range inner {
					result[key] = value
				}
			}
		case reflect.Pointer:
			ptrType := field.Type.Elem()
			if val.Field(index).IsNil() {
				result[name] = cty.UnknownVal(ctyType(ptrType.Kind()))
			} else {
				inner := reflect.ValueOf(val.Field(index).Interface())
				result[name] = primitive(ptrType.Kind(), inner.Elem())
			}
		default:
			result[name] = primitive(field.Type.Kind(), val.Field(index))
		}
	}

	return result
}

func primitive(kind reflect.Kind, field reflect.Value) cty.Value {
	switch kind {
	case reflect.String:
		return cty.StringVal(field.String())
	case reflect.Int:
		return cty.NumberIntVal(field.Int())
	}

	panic(fmt.Sprintf("unmapped native type %s", kind.String()))
}

func ctyType(kind reflect.Kind) cty.Type {
	switch kind {
	case reflect.String:
		return cty.String
	case reflect.Int:
		return cty.Number
	}

	panic(fmt.Sprintf("unmapped cty type %s", kind.String()))
}

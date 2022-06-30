package meta

import (
	"bake/internal/util"
	"fmt"
	"reflect"

	"github.com/hashicorp/hcl/v2"
)

func DecodeRange(body hcl.Body, ctx *hcl.EvalContext, val interface{}) hcl.Diagnostics {
	rv := reflect.ValueOf(val)
	if rv.Kind() != reflect.Ptr {
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("target value must be a pointer, not %s", rv.Type().String()),
		}}
	}

	return decodeBodyToValue(body, ctx, rv.Elem())
}

func decodeBodyToValue(body hcl.Body, ctx *hcl.EvalContext, val reflect.Value) hcl.Diagnostics {
	et := val.Type()
	switch et.Kind() {
	case reflect.Struct:
		return decodeBodyToStruct(body, ctx, val)
	default:
		return hcl.Diagnostics{{
			Severity: hcl.DiagError,
			Summary:  fmt.Sprintf("target value must be pointer to struct, not %s", et.String()),
		}}
	}
}

func decodeBodyToStruct(body hcl.Body, ctx *hcl.EvalContext, val reflect.Value) hcl.Diagnostics {
	attrs, diags := body.JustAttributes()
	if diags.HasErrors() {
		return diags
	}

	for index := 0; index < val.NumField(); index++ {
		field := val.Type().Field(index)
		fieldValue := val.Field(index)
		attr, ok := attrs[util.ToSnakeCase(field.Name)]
		if !ok {
			continue
		}

		fieldValue.Set(reflect.ValueOf(attr.Range))
	}

	return diags
}

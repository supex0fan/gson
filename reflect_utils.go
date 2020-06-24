package json

import (
	"reflect"
)

func isJsonArray(t reflect.Kind) bool {
	return t == reflect.Array || t == reflect.Slice
}

func isPrimitive(kind reflect.Kind) bool {
	switch kind {
	case reflect.Bool:
		fallthrough
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		fallthrough
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		fallthrough
	case reflect.Float32, reflect.Float64:
		fallthrough
	case reflect.String:
		return true
	}
	return false
}

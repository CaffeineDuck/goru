package executor

import (
	"encoding"
	"encoding/json"
	"reflect"
)

func encodeResponse(resp callResponse) []byte {
	budget := maxResponseBytes
	if responseFits(reflect.ValueOf(resp), &budget, 0) {
		if encoded, err := json.Marshal(resp); err == nil && len(encoded) < maxResponseBytes {
			return encoded
		}
	}
	encoded, _ := json.Marshal(callResponse{ID: resp.ID, Error: "response exceeds protocol limit or cannot be encoded"})
	return encoded
}

// Bound work before json.Marshal, which allocates a complete encoding even
// when used through json.Encoder. Callback values with custom encoders are
// trusted host code and must bound their own intermediate allocations.
func responseFits(v reflect.Value, budget *int, depth int) bool {
	*budget -= 32
	if *budget < 0 || depth > 64 {
		return false
	}
	if !v.IsValid() {
		return true
	}
	if v.CanInterface() {
		switch value := v.Interface().(type) {
		case json.RawMessage:
			if len(value) > *budget {
				return false
			}
			*budget -= len(value)
			return true
		case json.Number:
			*budget -= len(value)
			return *budget >= 0
		case json.Marshaler, encoding.TextMarshaler:
			return true
		}
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		return v.IsNil() || responseFits(v.Elem(), budget, depth+1)
	case reflect.String:
		if v.Len() > *budget/6 {
			return false
		}
		*budget -= 6 * v.Len()
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			if !responseFits(iter.Key(), budget, depth+1) || !responseFits(iter.Value(), budget, depth+1) {
				return false
			}
		}
	case reflect.Slice, reflect.Array:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			if v.Len() > *budget/4*3 {
				return false
			}
			*budget -= (v.Len() + 2) / 3 * 4
			return *budget >= 0
		}
		for i := 0; i < v.Len(); i++ {
			if !responseFits(v.Index(i), budget, depth+1) {
				return false
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if field.IsExported() && !responseFits(v.Field(i), budget, depth+1) {
				return false
			}
		}
	}
	return true
}

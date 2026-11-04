package engine

import (
	"fmt"
	"reflect"
	"strings"
)

// --- vars --------------------------------------------------------------------

// mergeVars folds layers left to right into a fresh map: maps merge key by key
// (recursively), scalars and lists replace, and a null value deletes the key.
// The result shares nothing with its inputs, so one project's vars can never
// leak into another's.
func mergeVars(layers ...map[string]any) map[string]any {
	out := map[string]any{}
	for _, l := range layers {
		mergeInto(out, l)
	}
	return out
}

func mergeInto(dst, src map[string]any) {
	for k, v := range src {
		if v == nil {
			delete(dst, k)
			continue
		}
		if sm, ok := asMap(v); ok {
			dm, ok := dst[k].(map[string]any)
			if !ok {
				dm = map[string]any{}
				dst[k] = dm
			}
			mergeInto(dm, sm)
			continue
		}
		dst[k] = cloneValue(v)
	}
}

// asMap normalises the two map shapes yaml.v3 produces.
func asMap(v any) (map[string]any, bool) {
	switch m := v.(type) {
	case map[string]any:
		return m, true
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, x := range m {
			out[fmt.Sprint(k)] = x
		}
		return out, true
	}
	return nil, false
}

func cloneValue(v any) any {
	if m, ok := asMap(v); ok {
		out := make(map[string]any, len(m))
		for k, x := range m {
			out[k] = cloneValue(x)
		}
		return out
	}
	if s, ok := v.([]any); ok {
		out := make([]any, len(s))
		for i, x := range s {
			out[i] = cloneValue(x)
		}
		return out
	}
	return v
}

// hasPath reports whether a dotted key path exists in a vars map. An existing
// key is present whatever its value ("" and 0 included); a missing key, or a
// path that runs through a non-map, is not. It is the `has` template func and
// what a bundle's `requires` entries are checked with.
func hasPath(v any, path string) bool {
	cur := v
	for _, seg := range strings.Split(path, ".") {
		m, ok := asMap(cur)
		if !ok {
			return false
		}
		if cur, ok = m[seg]; !ok {
			return false
		}
	}
	return path != ""
}

// isEmptyValue is the `default` func's notion of "not set": nil, the zero
// value of a scalar, or an empty list or map.
func isEmptyValue(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Map, reflect.Array, reflect.String:
		return rv.Len() == 0
	}
	return rv.IsZero()
}

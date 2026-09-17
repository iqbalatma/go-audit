package audit

import (
	"encoding/json"
	"reflect"
)

// toMap converts any struct/map into map[string]any via a JSON round-trip,
// so values compare consistently regardless of source type (struct field vs decoded JSON).
func toMap(v any) map[string]any {
	if v == nil {
		return nil
	}
	if m, ok := v.(map[string]any); ok {
		return m
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

func isNested(v any) bool {
	switch v.(type) {
	case map[string]any, []any:
		return true
	}
	return false
}

// diffMaps returns only the keys whose values differ between before and after.
// ponytail: nested array/object fields are skipped, not recursively diffed —
// upgrade to a recursive diff if you need to track changes inside JSON/relation fields.
func diffMaps(before, after map[string]any) (map[string]any, map[string]any) {
	diffBefore := map[string]any{}
	diffAfter := map[string]any{}

	keys := map[string]struct{}{}
	for k := range before {
		keys[k] = struct{}{}
	}
	for k := range after {
		keys[k] = struct{}{}
	}

	for k := range keys {
		bv, av := before[k], after[k]
		if isNested(bv) || isNested(av) {
			continue
		}
		if !reflect.DeepEqual(bv, av) {
			diffBefore[k] = bv
			diffAfter[k] = av
		}
	}
	return diffBefore, diffAfter
}

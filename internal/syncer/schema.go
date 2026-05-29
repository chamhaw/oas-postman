package syncer

import (
	"strings"
)

func sampleFromSchema(root map[string]any, schema map[string]any, explicit any) any {
	if explicit != nil {
		return explicit
	}
	return sampleSchema(root, schema, map[string]bool{}, 0)
}

func sampleSchema(root map[string]any, schema map[string]any, seen map[string]bool, depth int) any {
	if len(schema) == 0 || depth > 8 {
		return nil
	}
	if example, ok := schema["example"]; ok {
		return example
	}
	if def, ok := schema["default"]; ok {
		return def
	}
	if enum := asSlice(schema["enum"]); len(enum) > 0 {
		return enum[0]
	}
	if ref := asString(schema["$ref"]); ref != "" {
		if seen[ref] {
			return nil
		}
		seen[ref] = true
		resolved := resolveRef(root, ref)
		return sampleSchema(root, resolved, seen, depth+1)
	}
	if allOf := asSlice(schema["allOf"]); len(allOf) > 0 {
		merged := map[string]any{}
		hasObject := false
		for _, item := range allOf {
			if sampleMap, ok := sampleSchema(root, asMap(item), seen, depth+1).(map[string]any); ok {
				for k, v := range sampleMap {
					merged[k] = v
				}
				hasObject = true
			}
		}
		if hasObject {
			return merged
		}
	}
	for _, unionKey := range []string{"oneOf", "anyOf"} {
		if items := asSlice(schema[unionKey]); len(items) > 0 {
			return sampleSchema(root, asMap(items[0]), seen, depth+1)
		}
	}

	schemaType := asString(schema["type"])
	if schemaType == "" {
		if len(asMap(schema["properties"])) > 0 {
			schemaType = "object"
		} else if schema["items"] != nil {
			schemaType = "array"
		}
	}

	switch schemaType {
	case "object":
		out := map[string]any{}
		for _, key := range sortedKeys(asMap(schema["properties"])) {
			out[key] = sampleSchema(root, asMap(asMap(schema["properties"])[key]), seen, depth+1)
		}
		if len(out) == 0 {
			out["key_0"] = "string"
		}
		return out
	case "array":
		item := sampleSchema(root, asMap(schema["items"]), seen, depth+1)
		if item == nil {
			item = "string"
		}
		return []any{item}
	case "integer", "int", "long":
		return 0
	case "number", "float", "double":
		return 0
	case "boolean":
		return false
	case "string":
		switch strings.ToLower(asString(schema["format"])) {
		case "date-time":
			return "2026-01-01T00:00:00Z"
		case "date":
			return "2026-01-01"
		case "email":
			return "user@example.com"
		case "uuid":
			return "00000000-0000-0000-0000-000000000000"
		default:
			return "string"
		}
	default:
		return nil
	}
}

func resolveRef(root map[string]any, ref string) map[string]any {
	const prefix = "#/"
	if !strings.HasPrefix(ref, prefix) {
		return nil
	}
	parts := strings.Split(strings.TrimPrefix(ref, prefix), "/")
	var current any = root
	for _, part := range parts {
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")
		m := asMap(current)
		if m == nil {
			return nil
		}
		current = m[part]
	}
	return asMap(current)
}

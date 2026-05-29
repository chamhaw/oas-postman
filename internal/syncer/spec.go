package syncer

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func LoadDocument(path string) (Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Document{}, err
	}

	var root map[string]any
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&root); err != nil {
		return Document{}, fmt.Errorf("parse spec: %w", err)
	}
	if len(root) == 0 {
		return Document{}, fmt.Errorf("empty spec")
	}

	doc := Document{Root: root}
	info := asMap(root["info"])
	doc.Title = asString(info["title"])
	doc.Description = asString(info["description"])
	doc.Version = firstString(asString(root["openapi"]), asString(root["swagger"]))
	doc.Tags = parseTags(root["tags"])
	doc.BaseURL = parseBaseURL(root)

	paths := asMap(root["paths"])
	if len(paths) == 0 {
		return Document{}, fmt.Errorf("spec has no paths")
	}

	for _, specPath := range sortedKeys(paths) {
		pathItem := asMap(paths[specPath])
		if pathItem == nil {
			continue
		}
		pathParams := parseParameters(pathItem["parameters"])
		for _, method := range sortedKeys(pathItem) {
			if !operationMethods[strings.ToLower(method)] {
				continue
			}
			opMap := asMap(pathItem[method])
			if opMap == nil {
				continue
			}
			op := parseOperation(root, opMap, pathParams, strings.ToUpper(method), specPath)
			doc.Operations = append(doc.Operations, op)
		}
	}

	assignOperationOrders(&doc)
	return doc, nil
}

func parseTags(v any) []Tag {
	items := asSlice(v)
	tags := make([]Tag, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		m := asMap(item)
		name := asString(m["name"])
		if name == "" || seen[name] {
			continue
		}
		tags = append(tags, Tag{Name: name, Description: asString(m["description"])})
		seen[name] = true
	}
	return tags
}

func parseBaseURL(root map[string]any) string {
	if servers := asSlice(root["servers"]); len(servers) > 0 {
		if first := asMap(servers[0]); first != nil {
			if serverURL := asString(first["url"]); serverURL != "" {
				return serverURL
			}
		}
	}

	basePath := asString(root["basePath"])
	host := asString(root["host"])
	schemes := asStringSlice(root["schemes"])
	if host != "" {
		scheme := "https"
		if len(schemes) > 0 {
			scheme = schemes[0]
		}
		return strings.TrimRight(scheme+"://"+host, "/") + "/" + strings.TrimLeft(basePath, "/")
	}
	if basePath != "" {
		if !strings.HasPrefix(basePath, "/") {
			basePath = "/" + basePath
		}
		return basePath
	}
	return "/"
}

func parseOperation(root, opMap map[string]any, pathParams []Parameter, method, specPath string) Operation {
	tags := asStringSlice(opMap["tags"])
	op := Operation{
		ID:          asString(opMap["operationId"]),
		Method:      method,
		Path:        specPath,
		Tag:         "Default",
		Summary:     asString(opMap["summary"]),
		Description: asString(opMap["description"]),
		Security:    parseSecurity(root, opMap),
	}
	if len(tags) > 0 && tags[0] != "" {
		op.Tag = tags[0]
	}

	params := append([]Parameter{}, pathParams...)
	params = append(params, parseParameters(opMap["parameters"])...)
	op.Parameters = params
	op.RequestBody = parseRequestBody(root, opMap)
	op.Responses = parseResponses(root, opMap)
	return op
}

func parseParameters(v any) []Parameter {
	items := asSlice(v)
	out := make([]Parameter, 0, len(items))
	for _, item := range items {
		m := asMap(item)
		if m == nil {
			continue
		}
		p := Parameter{
			Name:        asString(m["name"]),
			In:          asString(m["in"]),
			Description: asString(m["description"]),
			Required:    asString(m["required"]) == "true",
			Schema:      asMap(m["schema"]),
			Example:     m["example"],
		}
		if p.Schema == nil {
			p.Schema = map[string]any{}
			for _, key := range []string{"type", "format", "enum", "items", "default"} {
				if value, ok := m[key]; ok {
					p.Schema[key] = value
				}
			}
		}
		if p.Name == "" || p.In == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func parseRequestBody(root, opMap map[string]any) *Payload {
	if requestBody := asMap(opMap["requestBody"]); requestBody != nil {
		if payload := parseContentPayload(requestBody["content"]); payload != nil {
			return payload
		}
	}

	for _, p := range parseParameters(opMap["parameters"]) {
		if p.In == "body" {
			return &Payload{
				ContentType: firstString(firstStringFromList(opMap["consumes"]), firstStringFromList(root["consumes"]), "application/json"),
				Schema:      p.Schema,
				Example:     p.Example,
			}
		}
	}
	return nil
}

func parseResponses(root, opMap map[string]any) []Response {
	responses := asMap(opMap["responses"])
	if len(responses) == 0 {
		return nil
	}
	statuses := sortedKeys(responses)
	out := make([]Response, 0, len(statuses))
	for _, status := range statuses {
		m := asMap(responses[status])
		if m == nil {
			continue
		}
		r := Response{
			Status:      status,
			Description: asString(m["description"]),
		}
		if payload := parseContentPayload(m["content"]); payload != nil {
			r.ContentType = payload.ContentType
			r.Schema = payload.Schema
			r.Example = payload.Example
		} else {
			r.ContentType = firstString(firstStringFromList(opMap["produces"]), firstStringFromList(root["produces"]), "application/json")
			r.Schema = asMap(m["schema"])
			if examples := asMap(m["examples"]); len(examples) > 0 {
				for _, key := range sortedKeys(examples) {
					r.Example = examples[key]
					break
				}
			}
		}
		out = append(out, r)
	}
	return out
}

func parseContentPayload(v any) *Payload {
	content := asMap(v)
	if len(content) == 0 {
		return nil
	}
	contentType := pickContentType(content)
	entry := asMap(content[contentType])
	if entry == nil {
		return nil
	}
	payload := &Payload{
		ContentType: contentType,
		Schema:      asMap(entry["schema"]),
		Example:     entry["example"],
	}
	if payload.Example == nil {
		if examples := asMap(entry["examples"]); len(examples) > 0 {
			for _, key := range sortedKeys(examples) {
				example := asMap(examples[key])
				if example != nil {
					payload.Example = example["value"]
				} else {
					payload.Example = examples[key]
				}
				break
			}
		}
	}
	return payload
}

func pickContentType(content map[string]any) string {
	if _, ok := content["application/json"]; ok {
		return "application/json"
	}
	if _, ok := content["application/*+json"]; ok {
		return "application/*+json"
	}
	keys := sortedKeys(content)
	if len(keys) == 0 {
		return "application/json"
	}
	return keys[0]
}

func parseSecurity(root, opMap map[string]any) *Security {
	securityValue, hasOperationSecurity := opMap["security"]
	if hasOperationSecurity {
		securityItems := asSlice(securityValue)
		if len(securityItems) == 0 {
			return &Security{}
		}
		return &Security{Basic: securityUsesBasic(root, securityItems)}
	}
	return &Security{Basic: securityUsesBasic(root, asSlice(root["security"]))}
}

func securityUsesBasic(root map[string]any, securityItems []any) bool {
	if len(securityItems) == 0 {
		return false
	}
	for _, item := range securityItems {
		requirement := asMap(item)
		for name := range requirement {
			if isBasicScheme(root, name) {
				return true
			}
		}
	}
	return false
}

func isBasicScheme(root map[string]any, name string) bool {
	swaggerSchemes := asMap(root["securityDefinitions"])
	if scheme := asMap(swaggerSchemes[name]); scheme != nil {
		return strings.EqualFold(asString(scheme["type"]), "basic")
	}
	components := asMap(root["components"])
	securitySchemes := asMap(components["securitySchemes"])
	if scheme := asMap(securitySchemes[name]); scheme != nil {
		return strings.EqualFold(asString(scheme["type"]), "http") && strings.EqualFold(asString(scheme["scheme"]), "basic")
	}
	return false
}

func firstStringFromList(v any) string {
	values := asStringSlice(v)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func assignOperationOrders(doc *Document) {
	tagIndex := map[string]int{}
	for i, tag := range doc.Tags {
		tagIndex[tag.Name] = i
	}
	sort.SliceStable(doc.Operations, func(i, j int) bool {
		left, right := doc.Operations[i], doc.Operations[j]
		li, lok := tagIndex[left.Tag]
		ri, rok := tagIndex[right.Tag]
		if lok && rok && li != ri {
			return li < ri
		}
		if lok != rok {
			return lok
		}
		if left.Tag != right.Tag {
			return left.Tag < right.Tag
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		return methodOrder[left.Method] < methodOrder[right.Method]
	})

	perTag := map[string]int{}
	for i := range doc.Operations {
		tag := doc.Operations[i].Tag
		perTag[tag]++
		doc.Operations[i].Order = perTag[tag] * 1000
	}
}

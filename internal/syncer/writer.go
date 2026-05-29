package syncer

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type collectionWriter struct {
	doc          Document
	options      Options
	existing     *existingCollection
	root         string
	usedRequests map[string]bool
	result       Result
	matched      map[*existingRequest]bool
}

func newCollectionWriter(doc Document, opts Options, existing *existingCollection, root string) *collectionWriter {
	return &collectionWriter{
		doc:          doc,
		options:      opts,
		existing:     existing,
		root:         root,
		usedRequests: map[string]bool{},
		matched:      map[*existingRequest]bool{},
		result:       Result{OutputDir: opts.OutputDir},
	}
}

func (w *collectionWriter) write() (Result, error) {
	if err := w.writeRootDefinition(); err != nil {
		return w.result, err
	}
	if err := w.writeOperations(); err != nil {
		return w.result, err
	}
	if w.options.OrphanPolicy == "deprecated" {
		if err := w.writeDeprecatedRequests(); err != nil {
			return w.result, err
		}
	}
	return w.result, nil
}

func (w *collectionWriter) writeRootDefinition() error {
	name := firstString(w.options.CollectionName, w.doc.Title, "OpenAPI")
	description := firstString(w.doc.Description, "Generated from OpenAPI/Swagger. Request definitions are regenerated; examples are preserved by oas-postman.")
	baseURL := firstString(w.options.BaseURL, w.doc.BaseURL, "/")
	def := CollectionDefinition{
		Kind:        "collection",
		Name:        name,
		Description: description,
		Variables: map[string]string{
			"baseUrl": baseURL,
		},
	}
	return writeYAMLFile(filepath.Join(w.root, ".resources", "definition.yaml"), def)
}

func (w *collectionWriter) writeOperations() error {
	folderDefs := w.folderDefinitions()
	for _, tag := range folderDefs {
		folderDir := filepath.Join(w.root, cleanName(tag.Name))
		def := CollectionDefinition{
			Kind:        "collection",
			Description: tag.Description,
			Order:       (tagIndex(w.doc.Tags, tag.Name) + 1) * 1000,
		}
		if err := writeYAMLFile(filepath.Join(folderDir, ".resources", "definition.yaml"), def); err != nil {
			return err
		}
	}

	for _, op := range w.doc.Operations {
		if err := w.writeOperation(op); err != nil {
			return err
		}
		w.result.OperationCount++
	}
	return nil
}

func (w *collectionWriter) folderDefinitions() []Tag {
	seen := map[string]Tag{}
	for _, tag := range w.doc.Tags {
		seen[tag.Name] = tag
	}
	for _, op := range w.doc.Operations {
		if _, ok := seen[op.Tag]; !ok {
			seen[op.Tag] = Tag{Name: op.Tag}
		}
	}
	out := make([]Tag, 0, len(seen))
	for _, tag := range w.doc.Tags {
		if value, ok := seen[tag.Name]; ok {
			out = append(out, value)
			delete(seen, tag.Name)
		}
	}
	for _, key := range sortedTagKeys(seen) {
		out = append(out, seen[key])
	}
	return out
}

func sortedTagKeys(tags map[string]Tag) []string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sortStrings(keys)
	return keys
}

func sortStrings(values []string) {
	if len(values) < 2 {
		return
	}
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func tagIndex(tags []Tag, name string) int {
	for i, tag := range tags {
		if tag.Name == name {
			return i
		}
	}
	return len(tags)
}

func (w *collectionWriter) writeOperation(op Operation) error {
	title := uniqueName(titleFromOperation(op), w.usedRequests)
	folderDir := filepath.Join(w.root, cleanName(op.Tag))
	resourceName := cleanName(title) + ".resources"
	examplesRel := "./.resources/" + resourceName + "/examples"
	examplesDir := filepath.Join(folderDir, ".resources", resourceName, "examples")

	request := w.requestFile(op, title, examplesRel)
	if err := writeYAMLFile(filepath.Join(folderDir, cleanName(title)+".request.yaml"), request); err != nil {
		return err
	}

	preserved := []exampleAsset{}
	if existing := w.existing.match(op); existing != nil {
		w.matched[existing] = true
		preserved = append(preserved, existing.Examples...)
	}
	count, err := writeExampleAssets(examplesDir, preserved)
	if err != nil {
		return err
	}
	w.result.PreservedExampleCount += count
	generated, err := w.writeGeneratedExamples(op, request, examplesDir, preserved)
	if err != nil {
		return err
	}
	w.result.GeneratedExampleCount += generated
	return nil
}

func (w *collectionWriter) requestFile(op Operation, title, examplesRel string) RequestFile {
	queryParams, pathVariables := parametersForRequest(w.doc.Root, op)
	headers := map[string]string{}
	if op.RequestBody != nil && op.RequestBody.ContentType != "" {
		headers["Content-Type"] = op.RequestBody.ContentType
	}
	if accept := acceptHeader(op); accept != "" {
		headers["Accept"] = accept
	}
	if len(headers) == 0 {
		headers = nil
	}

	body := Body{Type: "text", Content: ""}
	if op.RequestBody != nil {
		sample := sampleFromSchema(w.doc.Root, op.RequestBody.Schema, op.RequestBody.Example)
		body = Body{
			Type:    bodyType(op.RequestBody.ContentType),
			Content: prettyJSON(sample),
			Schema:  jsonSchemaString(op.RequestBody.Schema),
		}
	}

	var auth *Auth
	if op.Security != nil && op.Security.Basic {
		auth = &Auth{
			Type: "basic",
			Credentials: map[string]string{
				"username": "{{basicAuthUsername}}",
				"password": "{{basicAuthPassword}}",
			},
		}
	}

	return RequestFile{
		Kind:          "http-request",
		Name:          title,
		OperationID:   op.ID,
		Description:   firstString(op.Description, op.Summary),
		URL:           postmanURL("{{baseUrl}}", op.Path, queryParams),
		Method:        op.Method,
		Headers:       headers,
		QueryParams:   queryParams,
		PathVariables: pathVariables,
		Body:          body,
		Auth:          auth,
		Examples:      examplesRel,
		Order:         op.Order,
		Sync: SyncMeta{
			OperationID: op.ID,
			Method:      op.Method,
			Path:        normalizePathFromURL(op.Path),
		},
	}
}

func parametersForRequest(root map[string]any, op Operation) ([]KeyValue, []KeyValue) {
	var queryParams []KeyValue
	var pathVariables []KeyValue
	for _, param := range op.Parameters {
		value := valueString(param.Example, "string")
		if value == "string" && len(param.Schema) > 0 {
			if sample := sampleFromSchema(root, param.Schema, nil); sample != nil {
				value = valueString(sample, "string")
			}
		}
		item := KeyValue{Key: param.Name, Value: value, Description: param.Description}
		switch param.In {
		case "query":
			queryParams = append(queryParams, item)
		case "path":
			pathVariables = append(pathVariables, item)
		}
	}
	return queryParams, pathVariables
}

func acceptHeader(op Operation) string {
	for _, response := range op.Responses {
		if response.ContentType != "" {
			return response.ContentType
		}
	}
	return "application/json"
}

func bodyType(contentType string) string {
	if strings.Contains(contentType, "json") || contentType == "" {
		return "json"
	}
	if strings.HasPrefix(contentType, "text/") {
		return "text"
	}
	return "text"
}

func (w *collectionWriter) writeGeneratedExamples(op Operation, request RequestFile, examplesDir string, preserved []exampleAsset) (int, error) {
	count := 0
	for i, response := range op.Responses {
		name := cleanName(firstString(response.Description, statusText(response.Status), response.Status))
		fileName := name + ".example.yaml"
		if hasExampleNamed(preserved, fileName) {
			continue
		}
		statusCode, _ := strconv.Atoi(response.Status)
		if response.Status == "default" {
			statusCode = 0
		}
		headers := map[string]string{}
		if response.ContentType != "" && response.Status != "204" {
			headers["Content-Type"] = response.ContentType
		}
		if len(headers) == 0 {
			headers = nil
		}
		responseBody := Body{Type: "text", Content: ""}
		if response.Status != "204" {
			sample := sampleFromSchema(w.doc.Root, response.Schema, response.Example)
			responseBody = Body{
				Type:    bodyType(response.ContentType),
				Content: prettyJSON(sample),
				Schema:  jsonSchemaString(response.Schema),
			}
		}
		example := ExampleFile{
			Kind: "http-example",
			Name: name,
			Request: ExampleRequest{
				URL:           request.URL,
				Method:        request.Method,
				Headers:       request.Headers,
				QueryParams:   request.QueryParams,
				PathVariables: request.PathVariables,
				Body:          request.Body,
			},
			Response: ExampleResponse{
				StatusCode: statusCode,
				StatusText: statusText(response.Status),
				Headers:    headers,
				Body:       responseBody,
			},
			Order: statusOrder(response.Status, i),
			Sync: SyncMeta{
				OperationID: op.ID,
				Method:      op.Method,
				Path:        normalizePathFromURL(op.Path),
				Generated:   true,
				StatusCode:  response.Status,
			},
		}
		if err := writeYAMLFile(filepath.Join(examplesDir, fileName), example); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func (w *collectionWriter) writeDeprecatedRequests() error {
	used := map[string]bool{}
	for _, req := range w.existing.orphanables {
		if w.matched[req] {
			continue
		}
		name := uniqueName(firstString(req.Name, "Deprecated request"), used)
		folderDir := filepath.Join(w.root, "Deprecated")
		requestMap := cloneMap(req.RequestYAML)
		resourceName := cleanName(name) + ".resources"
		requestMap["name"] = name
		requestMap["examples"] = "./.resources/" + resourceName + "/examples"
		requestMap["x-postman-sync"] = map[string]any{
			"operationId": req.OperationID,
			"method":      req.Method,
			"path":        req.Path,
			"orphaned":    true,
		}
		if err := writeYAMLFile(filepath.Join(folderDir, cleanName(name)+".request.yaml"), requestMap); err != nil {
			return err
		}
		if _, err := writeExampleAssets(filepath.Join(folderDir, ".resources", resourceName, "examples"), req.Examples); err != nil {
			return err
		}
		w.result.DeprecatedCount++
	}
	if w.result.DeprecatedCount > 0 {
		def := CollectionDefinition{
			Kind:        "collection",
			Description: "Requests preserved from the previous collection that are no longer present in the current spec.",
			Order:       99000,
		}
		return writeYAMLFile(filepath.Join(w.root, "Deprecated", ".resources", "definition.yaml"), def)
	}
	return nil
}

func cloneMap(input map[string]any) map[string]any {
	out := make(map[string]any, len(input))
	for k, v := range input {
		out[k] = v
	}
	return out
}

func writeYAMLFile(path string, value any) error {
	data, err := marshalYAML(value)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

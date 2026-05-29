package syncer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyncPreservesExamplesByRouteWhenRequestNameChanges(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "swagger.yaml")
	outDir := filepath.Join(tmp, "collection")
	writeString(t, specPath, `
swagger: "2.0"
info:
  title: Example API
tags:
  - name: Things
    description: Thing operations
securityDefinitions:
  BasicAuth:
    type: basic
security:
  - BasicAuth: []
paths:
  /api/things/{id}:
    get:
      tags: [Things]
      summary: New thing title
      operationId: getThing
      parameters:
        - name: id
          in: path
          type: string
          required: true
      responses:
        "200":
          description: OK
          schema:
            type: object
            properties:
              id:
                type: string
`)
	writeString(t, filepath.Join(outDir, ".resources", "definition.yaml"), `$kind: collection
name: Old
`)
	writeString(t, filepath.Join(outDir, "Things", "Old thing title.request.yaml"), `$kind: http-request
name: Old thing title
url: "{{baseUrl}}/api/things/:id"
method: GET
examples: "./.resources/Old thing title.resources/examples"
`)
	writeString(t, filepath.Join(outDir, "Things", ".resources", "Old thing title.resources", "examples", "Manual.example.yaml"), `$kind: http-example
request:
  url: "{{baseUrl}}/api/things/:id"
  method: GET
response:
  statusCode: 200
  body:
    type: json
    content: '{"id":"manual"}'
`)

	result, err := Sync(Options{SpecPath: specPath, OutputDir: outDir})
	if err != nil {
		t.Fatal(err)
	}
	if result.OperationCount != 1 {
		t.Fatalf("operations = %d", result.OperationCount)
	}
	if result.PreservedExampleCount != 1 {
		t.Fatalf("preserved examples = %d", result.PreservedExampleCount)
	}

	request := readString(t, filepath.Join(outDir, "Things", "New thing title.request.yaml"))
	if !strings.Contains(request, "operationId: getThing") {
		t.Fatalf("request should include operationId metadata:\n%s", request)
	}
	manual := readString(t, filepath.Join(outDir, "Things", ".resources", "New thing title.resources", "examples", "Manual.example.yaml"))
	if !strings.Contains(manual, `"id":"manual"`) {
		t.Fatalf("manual example was not preserved:\n%s", manual)
	}
}

func TestLoadDocumentOpenAPI3(t *testing.T) {
	tmp := t.TempDir()
	specPath := filepath.Join(tmp, "openapi.yaml")
	writeString(t, specPath, `
openapi: 3.0.3
info:
  title: OAS3 API
servers:
  - url: https://example.com/v1
paths:
  /widgets:
    post:
      tags: [Widgets]
      summary: Create widget
      operationId: createWidget
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema:
                type: object
                properties:
                  id:
                    type: integer
`)
	doc, err := LoadDocument(specPath)
	if err != nil {
		t.Fatal(err)
	}
	if doc.BaseURL != "https://example.com/v1" {
		t.Fatalf("base URL = %q", doc.BaseURL)
	}
	if len(doc.Operations) != 1 || doc.Operations[0].ID != "createWidget" {
		t.Fatalf("unexpected operations: %#v", doc.Operations)
	}
	if doc.Operations[0].RequestBody == nil {
		t.Fatal("request body not parsed")
	}
}

func writeString(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimLeft(content, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

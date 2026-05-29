package syncer

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type existingCollection struct {
	byRoute     map[string]*existingRequest
	orphanables []*existingRequest
}

type existingRequest struct {
	Name        string
	Method      string
	Path        string
	OperationID string
	FilePath    string
	Deprecated  bool
	Examples    []exampleAsset
	RequestYAML map[string]any
}

type exampleAsset struct {
	Name string
	Data []byte
}

func collectExistingCollection(outputDir string) (*existingCollection, error) {
	out := &existingCollection{
		byRoute: map[string]*existingRequest{},
	}
	if _, err := os.Stat(outputDir); errors.Is(err, os.ErrNotExist) {
		return out, nil
	}

	err := filepath.WalkDir(outputDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(filePath, ".request.yaml") {
			return nil
		}
		req, err := readExistingRequest(outputDir, filePath)
		if err != nil {
			return fmt.Errorf("read existing request %s: %w", filePath, err)
		}
		out.orphanables = append(out.orphanables, req)
		if req.Method != "" && req.Path != "" {
			addExisting(out.byRoute, operationKey(req.Method, req.Path), req)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func addExisting(index map[string]*existingRequest, key string, req *existingRequest) {
	current := index[key]
	if current == nil || (current.Deprecated && !req.Deprecated) {
		index[key] = req
	}
}

func readExistingRequest(root, filePath string) (*existingRequest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	req := existingRequest{
		FilePath:    filePath,
		Deprecated:  strings.Contains(filepath.ToSlash(strings.TrimPrefix(filePath, root)), "/Deprecated/") || strings.Contains(filepath.ToSlash(filePath), "/Deprecated/"),
		RequestYAML: raw,
	}
	req.Name = firstString(asString(raw["name"]), strings.TrimSuffix(strings.TrimSuffix(filepath.Base(filePath), ".yaml"), ".request"))
	req.Method = strings.ToUpper(asString(raw["method"]))
	req.Path = normalizePathFromURL(asString(raw["url"]))
	req.OperationID = firstString(
		asString(raw["operationId"]),
		operationIDFromSyncMeta(raw["x-postman-sync"]),
	)

	examplesPath := asString(raw["examples"])
	if examplesPath == "" {
		return &req, nil
	}
	if !filepath.IsAbs(examplesPath) {
		examplesPath = filepath.Join(filepath.Dir(filePath), examplesPath)
	}
	req.Examples, err = readExampleAssets(examplesPath)
	if err != nil {
		return nil, err
	}
	return &req, nil
}

func operationIDFromSyncMeta(v any) string {
	return asString(asMap(v)["operationId"])
}

func readExampleAssets(dir string) ([]exampleAsset, error) {
	var assets []exampleAsset
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	err := filepath.WalkDir(dir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, filePath)
		if err != nil {
			return err
		}
		assets = append(assets, exampleAsset{Name: filepath.ToSlash(rel), Data: data})
		return nil
	})
	return assets, err
}

func (e *existingCollection) match(op Operation) *existingRequest {
	return e.byRoute[operationKey(op.Method, op.Path)]
}

func writeExampleAssets(dir string, assets []exampleAsset) (int, error) {
	count := 0
	used := map[string]int{}
	for _, asset := range assets {
		name := asset.Name
		if name == "" {
			continue
		}
		name = filepath.ToSlash(name)
		if !strings.HasSuffix(name, ".example.yaml") && !strings.HasSuffix(name, ".yaml") {
			name += ".example.yaml"
		}
		if seen := used[strings.ToLower(name)]; seen > 0 {
			ext := filepath.Ext(name)
			base := strings.TrimSuffix(name, ext)
			name = fmt.Sprintf("%s-preserved-%d%s", base, seen+1, ext)
		}
		used[strings.ToLower(name)]++
		target := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return count, err
		}
		if err := os.WriteFile(target, asset.Data, 0o644); err != nil {
			return count, err
		}
		count++
	}
	return count, nil
}

func hasExampleNamed(assets []exampleAsset, name string) bool {
	name = strings.ToLower(filepath.ToSlash(name))
	for _, asset := range assets {
		if strings.ToLower(filepath.ToSlash(asset.Name)) == name {
			return true
		}
	}
	return false
}

func marshalYAML(value any) ([]byte, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return nil, err
	}
	return bytes.TrimPrefix(data, []byte("---\n")), nil
}

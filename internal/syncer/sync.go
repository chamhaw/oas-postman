package syncer

import (
	"fmt"
	"os"
	"path/filepath"
)

func Sync(opts Options) (Result, error) {
	if opts.FolderStrategy == "" {
		opts.FolderStrategy = "tag"
	}
	if opts.OrphanPolicy == "" {
		opts.OrphanPolicy = "deprecated"
	}
	if err := validateOptions(opts); err != nil {
		return Result{}, err
	}

	doc, err := LoadDocument(opts.SpecPath)
	if err != nil {
		return Result{}, err
	}
	if opts.CollectionName == "" {
		opts.CollectionName = doc.Title
	}
	if opts.BaseURL == "" {
		opts.BaseURL = doc.BaseURL
	}

	existing, err := collectExistingCollection(opts.OutputDir)
	if err != nil {
		return Result{}, err
	}
	if opts.OrphanPolicy == "fail" {
		if err := failOnOrphans(doc, existing); err != nil {
			return Result{}, err
		}
	}

	parent := filepath.Dir(opts.OutputDir)
	base := filepath.Base(opts.OutputDir)
	tmpDir := filepath.Join(parent, ".oas-postman-sync-"+base)
	if err := os.RemoveAll(tmpDir); err != nil {
		return Result{}, err
	}

	writer := newCollectionWriter(doc, opts, existing, tmpDir)
	result, err := writer.write()
	if err != nil {
		return result, err
	}

	if opts.DryRun {
		_ = os.RemoveAll(tmpDir)
		return result, nil
	}
	if err := os.RemoveAll(opts.OutputDir); err != nil {
		return result, err
	}
	if err := os.Rename(tmpDir, opts.OutputDir); err != nil {
		return result, err
	}
	result.OutputDir = opts.OutputDir
	return result, nil
}

func validateOptions(opts Options) error {
	if opts.SpecPath == "" {
		return fmt.Errorf("spec path is required")
	}
	if opts.OutputDir == "" {
		return fmt.Errorf("output directory is required")
	}
	if opts.FolderStrategy == "" {
		opts.FolderStrategy = "tag"
	}
	if opts.FolderStrategy != "tag" {
		return fmt.Errorf("unsupported folder strategy %q: only tag is implemented", opts.FolderStrategy)
	}
	switch opts.OrphanPolicy {
	case "", "deprecated", "delete", "fail":
		return nil
	default:
		return fmt.Errorf("unsupported orphan policy %q", opts.OrphanPolicy)
	}
}

func failOnOrphans(doc Document, existing *existingCollection) error {
	current := map[string]bool{}
	for _, op := range doc.Operations {
		current["route:"+operationKey(op.Method, op.Path)] = true
	}
	for _, req := range existing.orphanables {
		if current["route:"+operationKey(req.Method, req.Path)] {
			continue
		}
		return fmt.Errorf("orphaned request %q (%s %s)", req.Name, req.Method, req.Path)
	}
	return nil
}

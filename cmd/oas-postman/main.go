package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/chamhaw/oas-postman/internal/syncer"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "oas-postman: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return nil
	}

	switch args[0] {
	case "sync":
		return runSync(args[1:])
	case "version", "--version", "-v":
		fmt.Printf("oas-postman %s commit=%s date=%s\n", version, commit, date)
		return nil
	case "help", "--help", "-h":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runSync(args []string) error {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	var opts syncer.Options
	fs.StringVar(&opts.SpecPath, "spec", "", "OpenAPI/Swagger JSON or YAML file")
	fs.StringVar(&opts.SpecPath, "s", "", "OpenAPI/Swagger JSON or YAML file")
	fs.StringVar(&opts.OutputDir, "out", "", "Postman v3 collection output directory")
	fs.StringVar(&opts.OutputDir, "o", "", "Postman v3 collection output directory")
	fs.StringVar(&opts.CollectionName, "name", "", "Postman collection name")
	fs.StringVar(&opts.BaseURL, "base-url", "", "baseUrl collection variable override")
	fs.StringVar(&opts.FolderStrategy, "folder-strategy", "tag", "folder strategy: tag")
	fs.StringVar(&opts.OrphanPolicy, "orphan-policy", "deprecated", "orphan policy: deprecated, delete, or fail")
	fs.BoolVar(&opts.DryRun, "dry-run", false, "parse and plan without writing files")
	fs.BoolVar(&opts.Verbose, "verbose", false, "print sync details")

	if err := fs.Parse(args); err != nil {
		return err
	}
	if opts.SpecPath == "" {
		return errors.New("--spec is required")
	}
	if opts.OutputDir == "" {
		return errors.New("--out is required")
	}

	result, err := syncer.Sync(opts)
	if err != nil {
		return err
	}

	fmt.Printf("synced %d operations into %s\n", result.OperationCount, result.OutputDir)
	if result.PreservedExampleCount > 0 {
		fmt.Printf("preserved %d existing examples\n", result.PreservedExampleCount)
	}
	if result.GeneratedExampleCount > 0 {
		fmt.Printf("generated %d spec examples\n", result.GeneratedExampleCount)
	}
	if result.DeprecatedCount > 0 {
		fmt.Printf("moved %d orphaned requests to Deprecated\n", result.DeprecatedCount)
	}
	if opts.DryRun {
		fmt.Println("dry run: no files written")
	}
	return nil
}

func printHelp() {
	fmt.Println(`oas-postman converts OpenAPI/Swagger specs into Postman v3 directory collections.

Usage:
  oas-postman sync --spec docs/swagger.json --out "postman/collections/API"
  oas-postman version

Sync behavior:
  - request definitions are regenerated from the spec
  - existing examples are matched only by method + normalized path
  - unmatched old requests are moved to Deprecated by default`)
}

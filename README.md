# oas-postman

`oas-postman` syncs an OpenAPI or Swagger spec into a Postman v3 directory collection.

It is built for the workflow where the spec is the source of truth for request definitions, while curated Postman examples are durable assets.

## Install

From source:

```sh
go install github.com/chamhaw/oas-postman/cmd/oas-postman@latest
```

Homebrew release publishing is prepared through GoReleaser. Once the tap exists:

```sh
brew tap chamhaw/tap
brew install --cask oas-postman
```

## Usage

```sh
oas-postman sync \
  --spec docs/swagger.json \
  --out "postman/collections/Sandbox Manager API" \
  --name "Sandbox Manager API"
```

The sync command:

- regenerates every request definition from the spec
- matches old examples by `operationId`, then by `METHOD + normalized path`
- moves old requests that no longer exist in the spec into `Deprecated/`
- writes `x-postman-sync` metadata into generated request/example YAML files

## Inputs

Supported spec inputs:

- Swagger 2.0 JSON/YAML
- OpenAPI 3.x JSON/YAML

Supported Postman output:

- Postman v3 directory collection
- tag-based folders
- request examples in `.resources/<request>.resources/examples`

## Release

Create a GitHub release by pushing a tag:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The release workflow uses GoReleaser to build binaries and publish a Homebrew cask into `chamhaw/homebrew-tap`. Set `TAP_GITHUB_TOKEN` with write access to that tap repository.

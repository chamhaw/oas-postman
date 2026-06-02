# oas-postman

`oas-postman` syncs an OpenAPI or Swagger spec into a Postman v3 directory collection.

It is built for the workflow where the spec is the source of truth for request definitions, while curated Postman examples are durable assets.

## Install

For local development, install SDKs with mise:

```sh
mise install
mise run check
```

For users installing the released CLI:

```sh
brew tap chamhaw/tap
brew install oas-postman
```

## Usage

```sh
oas-postman sync \
  --spec path/to/openapi.yaml \
  --out "postman/collections/My API" \
  --name "My API"
```

The sync command:

- regenerates every request definition from the spec
- matches old examples only by `METHOD + normalized path`
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

The release workflow creates a GitHub release and updates `chamhaw/homebrew-tap/Formula/oas-postman.rb`. Set `TAP_GITHUB_TOKEN` with write access to that tap repository.

Development and CI SDKs are managed by mise via `.mise.toml`. The Homebrew formula still uses Homebrew's normal Go build dependency because formula builds must be reproducible for users who do not have this repository's mise environment.

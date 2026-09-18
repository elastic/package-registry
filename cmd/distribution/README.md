# Distribution Tool

A utility for downloading packages from Elastic Package Registry (EPR).

## Overview

The distribution tool allows you to collect and download integration packages from an EPR instance based on configurable search queries. It supports filtering by package type, Kibana version, spec version, and other parameters.

## Building

```bash
cd cmd/distribution
go build
```

Or with the repository's build tool:

```bash
mage buildDistribution
```

Or install directly from any branch commit:

```bash
# latest from main
go install github.com/elastic/package-registry/cmd/distribution@latest

# reproducible pin
go install github.com/elastic/package-registry/cmd/distribution@<commit-sha>
```

Note: `@vX.Y.Z` version installs are not yet supported. The repository's
`vX.Y.Z` tags are root-module tags and do not apply to this nested module. Use
`@latest` or an explicit commit SHA.

## Usage

```bash
./distribution <config.yaml>
```

The tool requires a YAML configuration file that defines:
- **address**: EPR endpoint to query (defaults to `https://epr.elastic.co`)
- **keep**: Newest versions of each package to retain from each search response (default 0 = unlimited). When greater than 1, `all=true` is sent to EPR automatically so every version is returned before the window is applied. The window is per search response (one matrix entry × one query), so each Kibana release version gets its own set of newest installable versions before results are merged. Overridable per `matrix` entry or per `queries` entry.
- **queries**: Search parameters to filter packages
- **matrix**: Parameter combinations to expand queries
- **packages**: Specific packages to include by name and version
- **actions**: Operations to perform (print, download, validate)

See the `examples/` directory for complete configuration files (`all.yaml`,
`lite.yaml`, `pinned.yaml`, `sample.yaml`, `test.yaml`).

## Configuration Examples

### Minimal Configuration
```yaml
address: https://epr.elastic.co
queries:
  - package: nginx
actions:
  - print: {}
```

### Download with Validation
```yaml
address: https://epr.elastic.co
queries:
  - type: integration
    kibana.version: 8.0.0
actions:
  - download:
      destination: ./packages
      validate: true
```

## Actions

- **print**: Output package names and versions to console
- **download**: Download package ZIP files and signatures
  - `destination`: Target directory for downloads
  - `validate`: Verify package signatures using GPG

## Dependencies

`cmd/distribution` is a self-contained Go module with no dependency on the
parent `github.com/elastic/package-registry` module. The concurrency primitive
it needs (`internal/workers`) is a local copy, allowing `go install` to work
without any `replace` directives. Run `go mod tidy` inside this directory to
update dependencies.

# Distribution Tool

A utility for downloading packages from Elastic Package Registry (EPR).

## Overview

The distribution tool allows you to collect and download integration packages from an EPR instance based on configurable search queries. It supports filtering by package type, Kibana version, spec version, and other parameters.

## Building

```bash
cd cmd/distribution
go build
```

Or install directly:

```bash
go install github.com/elastic/package-registry/cmd/distribution@latest
```

## Usage

```bash
./distribution <config.yaml>
```

The tool requires a YAML configuration file that defines:
- **address**: EPR endpoint to query (defaults to `https://epr.elastic.co`)
- **queries**: Search parameters to filter packages
- **matrix**: Parameter combinations to expand queries
- **packages**: Specific packages to include by name and version
- **actions**: Operations to perform (print, download, validate)

See `minimal.yaml` and `lite-all.yaml` for example configurations.

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

Managed via Go modules. Run `go mod tidy` to update dependencies.

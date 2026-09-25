# Distribution Tool

A utility for downloading packages from Elastic Package Registry (EPR).

## Overview

The distribution tool allows you to collect and download integration packages from an EPR instance based on configurable search queries. It supports filtering by package type, Kibana version, spec version, and other parameters.

## Building

```bash
cd cmd/distribution
go build
```

Or with the repository's build tool (run from the repository root):

```bash
mage buildDistribution  # Build the distribution binary
mage test               # Run all tests (covers both modules)
mage check              # Format, license headers, mod tidy, staticcheck
mage modTidy            # Run go mod tidy across all modules
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
# Collect and download packages from a config file
./distribution <config.yaml>

# Add missing Kibana versions to a config's matrix
./distribution update-matrix <config.yaml>...
```

The tool requires a YAML configuration file that defines:
- **address**: EPR endpoint to query (defaults to `https://epr.elastic.co`)
- **keep**: Newest versions of each package to retain from each search response (default 0 = unlimited). When greater than 1, `all=true` is sent to EPR automatically so every version is returned before the window is applied. The window is per search response (one matrix entry × one query), so each Kibana release version gets its own set of newest installable versions before results are merged. Overridable per `matrix` entry or per `queries` entry.
- **queries**: Search parameters to filter packages
- **matrix**: Parameter combinations to expand queries
- **packages**: Specific package versions to include unconditionally (see below)
- **actions**: Operations to perform (print, download)

See the `examples/` directory for complete configuration files (`all.yaml`,
`lite.yaml`, `pinned.yaml`, `production-slim.yaml`, `sample.yaml`, `test.yaml`).

## Configuration Examples

### Minimal Configuration
```yaml
address: https://epr.elastic.co
queries:
  - package: nginx
actions:
  - print: {}
```

### Download Packages
```yaml
address: https://epr.elastic.co
queries:
  - type: integration
    kibana.version: 8.0.0
actions:
  - download:
      destination: ./packages
```

### Pinning Specific Versions

Use `packages:` to include exact package versions that search cannot reach —
versions older than the `keep` window, versions whose `conditions.kibana.version`
falls outside the matrix, or prerelease (`0.x`) versions that EPR only returns
under `prerelease=true`. Pinned entries bypass both search and the `keep` window.

```yaml
packages:
  - name: apache
    version: 0.1.3
  - name: endpoint
    version: 1.0.0
```

Pins are resolved directly to `epr/<name>/<name>-<version>.zip` on the configured
address. There is no existence check at config time; a wrong name or version causes
a hard failure at download time (non-200 response). Both the ZIP and its `.sig`
file are downloaded unconditionally — signature verification runs on every package.

See `examples/pinned.yaml` for a self-contained example.

## Actions

- **print**: Output package names and versions to console
- **download**: Download package ZIP files and signatures
  - `destination`: Target directory for downloads

## Updating the Kibana matrix

The `update-matrix` subcommand keeps the `matrix:` block of one or more config
files up to date with the currently active Elastic branches and their next
planned versions.

```bash
./distribution update-matrix examples/production-slim.yaml examples/lite.yaml
```

**What it does:**

1. Fetches the list of active branches from the Elastic artifacts API:
   `https://storage.googleapis.com/artifacts-api/snapshots/branches.json`
   (e.g. `["main", "9.5", "9.4", "8.19"]`)

2. For each branch, fetches the next planned version (with `-SNAPSHOT` stripped)
   from `.../snapshots/<branch>.json`.

3. Fetches the Fleet spec bounds for that branch from
   `https://raw.githubusercontent.com/elastic/kibana/<branch>/x-pack/platform/plugins/shared/fleet/server/config.ts`.
   `REGISTRY_SPEC_MAX_VERSION` is required; `REGISTRY_SPEC_MIN_VERSION` is
   optional (some older branches omit it).

4. Generates one matrix entry for every patch from `X.Y.0` through the next
   version inclusive (e.g. next `9.5.5` → entries 9.5.0–9.5.5).

5. **Merges by adding only** — existing entries are never modified or removed.
   The matrix intentionally covers all 8.x and 9.x releases (7.x is the only
   floor removed by hand). Removing versions is always a manual change in a
   reviewed PR.

6. Rewrites the file in place, preserving header comments, other sections, and
   commented-out entries inside the matrix block.

The command is idempotent: a second run with the same active branches produces
no diff.

## Dependencies

`cmd/distribution` is a self-contained Go module with no dependency on the
parent `github.com/elastic/package-registry` module. The concurrency primitive
it needs (`internal/workers`) is a local copy, allowing `go install` to work
without any `replace` directives. Run `mage modTidy` from the repository root
(or `go mod tidy` inside this directory) to update dependencies.

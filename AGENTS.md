# Elastic Package Registry - AI Assistant Instructions

## Build, Test, and Lint

This project uses [Mage](https://magefile.org) as its build tool. Run `mage` without arguments to see all available targets.

### Common Commands

```bash
# Build
mage build                # Build package-registry binary
mage buildFIPS            # Build with FIPS 140 support
go run .                  # Run directly without building

# Test
mage test                 # Run all tests
mage testFIPS             # Run tests with FIPS 140 enabled
go test ./...             # Alternative: run all tests
go test -v ./packages     # Run tests for specific package
go test . -generate       # Regenerate golden test files (after adding test packages)

# Lint/Check
mage check                # Run all checks (format, license headers, mod tidy, staticcheck)
mage format               # Format code and add license headers
mage staticcheck          # Run static analysis

# Clean
mage clean                # Remove build artifacts
```

### Docker

```bash
mage dockerBuild main     # Build Docker image with tag "main"
docker run --rm -it -p 8080:8080 docker.elastic.co/package-registry/package-registry:main
```

## Architecture Overview

### Core Concepts

**Package Registry** is an HTTP service that serves Elastic integration packages to Kibana/Fleet. It has two main phases:

1. **Indexing**: Loading and indexing package metadata from various sources
2. **Serving**: HTTP endpoints to search and retrieve packages

### Key Components

**Indexers** (`indexer.go`, `storage/indexer.go`)
- Abstract interface for loading packages from different sources
- Multiple indexers can be combined via `CombinedIndexer`
- Three implementations:
  - **File System Indexer**: Reads packages from local directories/zip files (default)
  - **Storage Indexer**: Loads packages from Google Cloud Storage into memory
  - **SQL Storage Indexer**: Loads packages from GCS into SQLite (technical preview)

**Proxy Mode** (`proxymode/`)
- Allows EPR to forward requests to another registry endpoint (e.g., production EPR)
- Combines local packages with remote packages
- Useful for development/testing with production package data
- Enable with `-feature-proxy-mode=true` and `-proxy-to=https://epr.elastic.co`

**Packages** (`packages/`)
- Core data structures for package metadata (`Package`, `DataStream`, `PolicyTemplate`)
- Handles package versioning with semantic versioning
- Supports package discovery, filtering, and resolution

**Handlers** (`handler.go`, `search.go`, `categories.go`, `index.go`)
- HTTP endpoint implementations
- Most endpoints serve pre-generated static content
- `/search` and `/categories` are dynamic (support query parameters)
- Cache headers configured per endpoint

### Configuration

- Config file: `config.yml` (see `config.reference.yml` for all options)
- Flags can be set via environment variables: `EPR_<FLAG_NAME_UPPERCASE>`
  - Example: `EPR_DRY_RUN=true` is equivalent to `-dry-run`
- Default config includes cache times, paths, and feature flags

### Testing

Test packages are stored in `testdata/package/` with various scenarios (e.g., `defaultrelease`, `dataset_is_prefix`, `agent_version`). When adding new test packages:

1. Add the package directory under `testdata/package/`
2. Run `mage writeTestGoldenFiles` to regenerate expected test outputs
3. Commit both the test package and updated golden files

## Conventions

### Code Style

- **Tabs for indentation** in Go files (enforced by `.editorconfig`)
- **License headers** required on all `.go` files (use `mage format` to add)
- **Import grouping**: Elastic packages (`github.com/elastic`) grouped after third-party
- Run `mage check` before committing to ensure formatting and licensing

### Testing Assertions

When using testify for test assertions:
- Use **`require`** only for blocking assertions that would cause panics if they fail (e.g., nil checks before dereferencing)
- Use **`assert`** for all other assertions so multiple checks can run in the same test
- This approach provides better test failure visibility by showing all failing assertions at once

Example:
```go
// Use require for blocking checks
require.NotNil(t, result)           // Must pass or dereferencing will panic
require.NoError(t, err)             // Must pass or result may be invalid

// Use assert for validation checks
assert.Equal(t, "expected", result.Name)
assert.True(t, result.IsValid)
assert.Len(t, result.Items, 3)
```

### Package Structure

Packages follow the [package-spec](https://github.com/elastic/package-spec) specification. Changes to package structure should be proposed to package-spec first.

### Storage Indexers

When working with Storage Indexers (cloud storage backend):
- Use `dev/launch_fake_gcs_server.sh` to test locally with fake GCS
- Use `dev/launch_epr_service_storage_indexer.sh` to run EPR with storage indexer enabled
- Test data: `storage/testdata/search-index-all-full.json`

### Release Process

Releases are tagged via GitHub releases. After tagging:
1. CI builds and publishes Docker image as `docker.elastic.co/package-registry/package-registry:vX.Y.Z`
2. Update version in dependent projects (elastic/integrations, elastic/elastic-package)
3. See README "Release" section for full checklist

### Monitoring

- **APM**: Instrumented with Elastic APM Go Agent (configure via `ELASTIC_APM_*` env vars)
- **Metrics**: Prometheus metrics available at `/metrics` (enable with `-metrics-address`)
- **Profiling**: HTTP profiler available with `-httpprof` flag
- **Debugging**: Enable with `-log-level debug` or `EPR_LOG_LEVEL=debug`

## Version

Current version: Check `version` constant in `main.go` (currently 1.35.1)

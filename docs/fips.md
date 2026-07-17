# FIPS 140-3

FIPS 140-3 is a US/Canadian government standard that certifies cryptographic
modules meet a baseline of security requirements; it's widely required for
software deployed in government, defense, financial, and other regulated
environments. Package Registry needs to support it so that it can be deployed
as part of the Elastic Stack in those FIPS-regulated customer environments.

Package Registry can be built against Go's certified FIPS 140-3 crypto module,
for use in environments that require FIPS 140-3 enforcement.

See [Go's FIPS 140-3 documentation](https://go.dev/doc/security/fips140#fips-140-3-mode)
for background on `GOFIPS140` and the native Go crypto module. The module used
is pinned to `v1.0.0`:

* CMVP Certificate [#4735](https://csrc.nist.gov/projects/cryptographic-module-validation-program/certificate/4735)
* CAVP Certificate [A6650](https://csrc.nist.gov/projects/cryptographic-algorithm-validation-program/details?product=19371)

## Building a FIPS 140-3 binary

Set `GOFIPS140=v1.0.0` when building, this pins the build to the certified
module version instead of whatever is bundled with the Go toolchain in use.

Using `mage`:
```bash
mage buildFIPS
```

Using `make` (used to build the release binaries and the Docker image):
```bash
make release-linux FIPS=1
```

Using `go build` directly:
```bash
GOFIPS140=v1.0.0 go build .
```

## Building the FIPS 140-3 Docker image

```bash
mage dockerBuildFIPS main
```

This builds `docker.elastic.co/package-registry/package-registry:main-fips`.
Published FIPS images use the same `-fips` tag suffix as the existing `-ubi`
variant, for example `docker.elastic.co/package-registry/package-registry:v1.40.0-fips`.

Package-filled distribution images are also published with the `-fips` suffix.
For an Elastic Stack release such as `8.19.0`, the production and lite variants
are available as:

```text
docker.elastic.co/package-registry/distribution:8.19.0-fips
docker.elastic.co/package-registry/distribution:lite-8.19.0-fips
```

These images use the FIPS-enabled Package Registry server and contain the same
package sets as their corresponding non-FIPS distribution images. The
`cmd/distribution` limitation described below applies to the build-time package
collection tool, not to the deployed server in these images.

## Verifying a binary

A binary built with `GOFIPS140` embeds that information, along with a default
`GODEBUG=fips140=on` setting, in its build info:

```bash
go version -m ./package-registry | grep -E 'GOFIPS140|DefaultGODEBUG'
# build	DefaultGODEBUG=fips140=on
# build	GOFIPS140=v1.0.0-<...>
```

With `fips140=on` (the default for FIPS builds), the module is used for all
supported crypto operations and non-approved algorithms remain available as a
fallback. Operators that need strict enforcement, where non-approved
algorithms are refused instead of falling back, can set `GODEBUG=fips140=only`
in the environment. See [Known limitations](#known-limitations) below before
doing so.

## What is covered

Package Registry itself does not implement any custom cryptography. Its own
runtime code paths only rely on the standard library's `crypto/tls`, used for
serving HTTPS (`-tls-cert`/`-tls-key`) and for TLS client connections to
backing package storage. Neither `package-registry` nor the real
`cloud.google.com/go/storage` client it uses in production call `crypto/md5`
or other non-approved algorithms.

`crypto/tls`'s default minimum version is TLS 1.2, which satisfies FIPS 140-3.
FIPS builds reject startup with an error if `-tls-min-version` (or
`EPR_TLS_MIN_VERSION`) is set below TLS 1.2, since TLS 1.1 is not FIPS 140-3
approved.

The test suite passes in full under strict `GODEBUG=fips140=only` (see
`mage testFIPS`). The test-only GCS emulator (`github.com/fsouza/fake-gcs-server`,
also used at runtime by the built-in fake GCS server behind
`EPR_EMULATOR_INDEX_PATH`) pre-populates each fake object's MD5 field so the
library never has to hash content with `crypto/md5` itself; see
`internal/storage/fakestorage.go`.

## Known limitations

### `cmd/distribution` is not FIPS-covered

`cmd/distribution` is a release tooling binary (not the deployed server) that
downloads packages from EPR and verifies their signatures against Elastic's
release-signing key. It is not subject to FIPS 140-3 requirements and has no
FIPS build target.

The blocker is structural: Elastic's release-signing key is an OpenPGP v4 key,
and RFC 4880 §12.2 mandates SHA-1 for all v4 key fingerprints. The
`github.com/ProtonMail/go-crypto` library computes this SHA-1 unconditionally
at parse time whenever it reads any v4 key. Under `GODEBUG=fips140=only` that
call panics immediately, before any package verification takes place.

Resolving this requires Elastic to re-issue the release-signing key as an
OpenPGP v6 key (which uses SHA-256 fingerprints). That is an org-level change
outside the scope of this repository. Note that the SHA-1 is only used for the
key's identifier; actual package integrity verification uses SHA-256.

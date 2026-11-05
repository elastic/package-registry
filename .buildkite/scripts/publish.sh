#!/bin/bash
source .buildkite/scripts/tooling.sh
source .buildkite/scripts/pre-install-command.sh
source .buildkite/scripts/publish.helpers.sh

set -euo pipefail

set_image_tags

# Create a single buildx builder reused by the three flavour builds below.
setup_buildx

echo "--- Build package-registry"
build_push_docker_image "docker.elastic.co/wolfi/chainguard-base" ""
echo "--- Build package-registry UBI"
build_push_docker_image "registry.access.redhat.com/ubi9/ubi-minimal:9.6" "-ubi"
echo "--- Build package-registry FIPS"
build_push_docker_image "docker.elastic.co/wolfi/chainguard-base-fips" "-fips"

# Resolve each pushed image's digest and upload it for the unified-release-container-image-signing
# pipeline. crane reads the digest from the registry, so it is only installed and used on the push
# path. PR builds run this too so the whole path is exercised on every PR, but their digest files are
# named so the signing pipeline cannot collect them (see upload_image_digest_for_signing); the signing
# trigger steps in .buildkite/pipeline.yml are gated off for PRs as well, so PR images are not signed.
with_crane
echo "--- Create digest files for package-registry images"
upload_image_digest_for_signing "base" ""
upload_image_digest_for_signing "ubi" "-ubi"
upload_image_digest_for_signing "fips" "-fips"

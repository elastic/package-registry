#!/bin/bash
source .buildkite/scripts/tooling.sh
source .buildkite/scripts/pre-install-command.sh
source .buildkite/scripts/publish.helpers.sh

set -euo pipefail

set_image_tags

# jq is needed to read the pushed image digest out of the buildx metadata files.
add_bin_path
with_jq

# Create a single buildx builder reused by the three flavour builds below.
setup_buildx

echo "--- Build package-registry"
build_push_docker_image "docker.elastic.co/wolfi/chainguard-base" ""
echo "--- Build package-registry UBI"
build_push_docker_image "registry.access.redhat.com/ubi9/ubi-minimal:9.6" "-ubi"
echo "--- Build package-registry FIPS"
build_push_docker_image "docker.elastic.co/wolfi/chainguard-base-fips" "-fips"

# Capture each pushed image's multi-arch index digest from the buildx metadata (immutable, resolved at
# push time, so no registry read-back and no race with a concurrent build repointing a tag) and upload
# one digest file per flavour for the unified-release-container-image-signing pipeline. PR builds do
# this too, under a non-collectable "pr-" name (see digest_file_name); the signing trigger steps in
# .buildkite/pipeline.yml are gated off for PRs.
echo "--- Create digest files for package-registry images"
upload_image_digest_for_signing "base" ""
upload_image_digest_for_signing "ubi" "-ubi"
upload_image_digest_for_signing "fips" "-fips"

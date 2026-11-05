#!/bin/bash
source .buildkite/scripts/tooling.sh
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

# Upload a build-unique image reference per flavour for the separate digest step
# (resolve_image_digests.sh) to resolve after the push. Digest resolution is decoupled from the push
# so a transient crane/registry failure cannot fail an already-successful push (which would leave the
# pushed image unsigned); the digest step is retried on its own instead. The refs are frozen in the
# artifacts so the digest step resolves the exact images this build pushed, independent of any shared
# tag. PR builds upload refs too (the digest step resolves them under a non-collectable "pr-" name to
# exercise the path); the signing trigger steps in .buildkite/pipeline.yml are gated off for PRs.
echo "--- Upload image refs for signing"
upload_image_ref_for_signing "base" ""
upload_image_ref_for_signing "ubi" "-ubi"
upload_image_ref_for_signing "fips" "-fips"

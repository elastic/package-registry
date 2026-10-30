#!/bin/bash
source .buildkite/scripts/tooling.sh
source .buildkite/scripts/pre-install-command.sh
source .buildkite/scripts/publish.helpers.sh

set -euo pipefail

set_image_tags

build_push_docker_image "docker.elastic.co/wolfi/chainguard-base" ""
build_push_docker_image "registry.access.redhat.com/ubi9/ubi-minimal:9.6" "-ubi"
build_push_docker_image "docker.elastic.co/wolfi/chainguard-base-fips" "-fips"

# Sign non-PR builds (branch builds and release tags): resolve each pushed image's digest and upload
# it for the unified-release-container-image-signing pipeline. crane reads the digest from the
# registry, so it is only installed and used on the push path; PR images are not signed.
if [[ "${BUILDKITE_PULL_REQUEST}" == "false" ]]; then
	with_crane
	upload_image_digest_for_signing "base" ""
	upload_image_digest_for_signing "ubi" "-ubi"
	upload_image_digest_for_signing "fips" "-fips"
fi

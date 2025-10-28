#!/bin/bash
source .buildkite/scripts/tooling.sh

set -euo pipefail

build_push_docker_image() {
	local runner_image="${1}"
	local tag_suffix="${2:-}"
	local docker_img_tag="${DOCKER_IMG_TAG}${tag_suffix}"
	local docker_img_tag_branch="${DOCKER_IMG_TAG_BRANCH}${tag_suffix}"
	go_version=$(cat .go-version)

	docker buildx create --use

	retry 3 docker buildx build --push \
		--platform linux/amd64,linux/arm64/v8 \
		--progress plain \
		-t "${docker_img_tag}" \
		-t "${docker_img_tag_branch}" \
		--build-arg GO_VERSION="${go_version}" \
		--build-arg BUILDER_IMAGE=docker.elastic.co/wolfi/go \
		--build-arg RUNNER_IMAGE="${runner_image}" \
		--label BRANCH_NAME="${TAG_NAME}" \
		--label GIT_SHA="${BUILDKITE_COMMIT}" \
		--label GO_VERSION="${SETUP_GOLANG_VERSION}" \
		--label TIMESTAMP="$(date +%Y-%m-%d_%H:%M)" \
		.

	echo "Docker images pushed: ${DOCKER_IMG_TAG}${tag_suffix} ${DOCKER_IMG_TAG_BRANCH}${tag_suffix}"
}

if [[ "${BUILDKITE_PULL_REQUEST}" != "false" ]]; then
	DOCKER_NAMESPACE="${DOCKER_IMG_PR}"
	TAG_NAME="PR-${BUILDKITE_PULL_REQUEST}"
else
	DOCKER_NAMESPACE="${DOCKER_IMG}"
	TAG_NAME="${BUILDKITE_BRANCH}" # e.g. main
fi

# if tag exists use tag instead
if [ -n "${BUILDKITE_TAG:-}" ]; then
	TAG_NAME="${BUILDKITE_TAG}"
fi

DOCKER_IMG_TAG="${DOCKER_NAMESPACE}:${BUILDKITE_COMMIT}"
DOCKER_IMG_TAG_BRANCH="${DOCKER_NAMESPACE}:${TAG_NAME}"

build_push_docker_image "docker.elastic.co/wolfi/chainguard-base" ""
build_push_docker_image "registry.access.redhat.com/ubi9/ubi-minimal:9.6" "-ubi"

#!/bin/bash
source .buildkite/scripts/tooling.sh

set -euo pipefail

build_docker_image() {
    local go_version
    go_version=$(cat .go-version)

    docker buildx build "$@" \
        --platform linux/amd64,linux/arm64/v8 \
        -t "${DOCKER_IMG_TAG}" \
        -t "${DOCKER_IMG_TAG_BRANCH}" \
        --build-arg GO_VERSION="${go_version}" \
        --build-arg BUILDER_IMAGE=docker.elastic.co/wolfi/go \
        --build-arg RUNNER_IMAGE=docker.elastic.co/wolfi/chainguard-base \
        --label BRANCH_NAME="${TAG_NAME}" \
        --label GIT_SHA="${BUILDKITE_COMMIT}" \
        --label GO_VERSION="${SETUP_GOLANG_VERSION}" \
        --label TIMESTAMP="$(date +%Y-%m-%d_%H:%M)" \
        .
}

push_docker_image() {
    docker buildx create --use

    # first build the image without push
    build_docker_image

    # essentially the same as above with --push flag; the build should be in the cache
    retry 3 build_docker_image --push

    echo "Docker images pushed: ${DOCKER_IMG_TAG} ${DOCKER_IMG_TAG_BRANCH}"
}

if [[ "${BUILDKITE_PULL_REQUEST}" != "false" ]]; then
    DOCKER_NAMESPACE="${DOCKER_IMG_PR}"
    TAG_NAME="PR-${BUILDKITE_PULL_REQUEST}"
else
    DOCKER_NAMESPACE="${DOCKER_IMG}"
    TAG_NAME="${BUILDKITE_BRANCH}"  # e.g. main
fi

# if tag exists use tag instead
if [ -n "${BUILDKITE_TAG:-}" ]; then
    TAG_NAME="${BUILDKITE_TAG}"
fi

DOCKER_IMG_TAG="${DOCKER_NAMESPACE}:${BUILDKITE_COMMIT}"
DOCKER_IMG_TAG_BRANCH="${DOCKER_NAMESPACE}:${TAG_NAME}"

push_docker_image

#!/bin/bash
source .buildkite/scripts/tooling.sh

set -euo pipefail


pushDockerImage() {
    docker buildx create --use
    docker buildx build --push \
        --platform linux/amd64,linux/arm64 \
        -t "${DOCKER_IMG_TAG}" \
        -t "${DOCKER_IMG_TAG_BRANCH}" \
        --label BRANCH_NAME="${TAG_NAME}" \
        --label GIT_SHA="${BUILDKITE_COMMIT}" \
        --label GO_VERSION="${SETUP_GOLANG_VERSION}" \
        --label TIMESTAMP="$(date +%Y-%m-%d_%H:%M)" \
        .

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

pushDockerImage

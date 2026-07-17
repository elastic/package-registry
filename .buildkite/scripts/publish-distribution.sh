#!/bin/bash
# DRY_RUN is true by default - script will not push to registry
source .buildkite/scripts/tooling.sh

set -euo pipefail

DOCKER_TAG=$(buildkite-agent meta-data get DOCKER_TAG --default="")
if [[ -z "${DOCKER_TAG:-""}" ]]; then
    echo "ERROR: DOCKER_TAG meta-data is empty or not set"
    exit 1
fi

echo "version for tagging: ${DOCKER_TAG}"

DOCKER_IMG_SOURCE="${DOCKER_IMAGE}:${TAG_NAME}"

# DOCKER_IMAGE_RENAMED is optional: when set, the retagged image is also
# published under this second name (same digest), e.g. to introduce a new
# image name in docker.elastic.co without breaking existing consumers of
# DOCKER_IMAGE.
DOCKER_IMAGE_TARGETS=("${DOCKER_IMAGE}")
if [[ -n "${DOCKER_IMAGE_RENAMED:-""}" ]]; then
    DOCKER_IMAGE_TARGETS+=("${DOCKER_IMAGE_RENAMED}")
fi
IMAGE_SUFFIXES=("" "-ubi" "-fips")

echo "Docker retag"
docker buildx create --use

for image in "${DOCKER_IMAGE_TARGETS[@]}"; do
    if [[ "${TAG_NAME}" == "production" ]]; then
        DOCKER_IMG_TARGET="${image}:${DOCKER_TAG}"
    else
        DOCKER_IMG_TARGET="${image}:${TAG_NAME}-${DOCKER_TAG}"
    fi

    for suffix in "${IMAGE_SUFFIXES[@]}"; do
        source_image="${DOCKER_IMG_SOURCE}${suffix}"
        target_image="${DOCKER_IMG_TARGET}${suffix}"

        # do not push if DRY_RUN is true
        if [[ ${DRY_RUN:-true} == "true" ]]; then
            docker buildx imagetools create --dry-run -t "${target_image}" "${source_image}"
        else
            retry 3 docker buildx imagetools create -t "${target_image}" "${source_image}"
            echo "Docker image pushed: ${target_image}"
        fi
    done
done

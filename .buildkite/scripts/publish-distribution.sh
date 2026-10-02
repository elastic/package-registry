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
IMAGE_SUFFIXES=("" "-ubi")

# Slim distributions do not exist until the first build that includes them.
# Skip the whole retag during that transition.
if [[ "${TAG_NAME}" == *-slim ]]; then
    if retry 3 docker buildx imagetools inspect "${DOCKER_IMG_SOURCE}" > /dev/null; then
        buildkite-agent meta-data set "slim-distribution-${TAG_NAME}-available" "true"
    else
        echo "Slim source image is not available yet, skipping: ${DOCKER_IMG_SOURCE}"
        buildkite-agent meta-data set "slim-distribution-${TAG_NAME}-available" "false"
        buildkite-agent meta-data set "fips-distribution-${TAG_NAME}-available" "false"
        exit 0
    fi
fi

FIPS_SOURCE_IMAGE="${DOCKER_IMG_SOURCE}-fips"
FIPS_SOURCE_AVAILABLE=false

# The FIPS distribution source does not exist until the first release that
# includes a FIPS package-registry image. Skip it during that transition.
if retry 3 docker buildx imagetools inspect "${FIPS_SOURCE_IMAGE}" > /dev/null; then
    IMAGE_SUFFIXES+=("-fips")
    FIPS_SOURCE_AVAILABLE=true
else
    echo "FIPS source image is not available yet, skipping: ${FIPS_SOURCE_IMAGE}"
fi

echo "Docker retag"
docker buildx create --use

for image in "${DOCKER_IMAGE_TARGETS[@]}"; do
    # NOTE: the non-slim tags (lite*, production*) are planned to be deprecated
    # in favour of the slim ones (*-slim*). Keep retagging both until then.
    case "${TAG_NAME}" in
        production)
            DOCKER_IMG_TARGET="${image}:${DOCKER_TAG}"
            ;;
        production-slim)
            DOCKER_IMG_TARGET="${image}:${DOCKER_TAG}-slim"
            ;;
        *-slim)
            DOCKER_IMG_TARGET="${image}:${TAG_NAME%-slim}-${DOCKER_TAG}-slim"
            ;;
        *)
            DOCKER_IMG_TARGET="${image}:${TAG_NAME}-${DOCKER_TAG}"
            ;;
    esac

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

buildkite-agent meta-data set "fips-distribution-${TAG_NAME}-available" "${FIPS_SOURCE_AVAILABLE}"

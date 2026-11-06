#!/bin/bash
# DRY_RUN is true by default - script will not push to registry
source .buildkite/scripts/tooling.sh
source .buildkite/scripts/pre-install-command.sh
source .buildkite/scripts/publish.helpers.sh
source .buildkite/scripts/publish-distribution.helpers.sh

set -euo pipefail

DOCKER_TAG=$(buildkite-agent meta-data get DOCKER_TAG --default="")
if [[ -z "${DOCKER_TAG:-""}" ]]; then
    echo "ERROR: DOCKER_TAG meta-data is empty or not set"
    exit 1
fi

echo "version for tagging: ${DOCKER_TAG}"

# SKIP_RETAG (default false): when "true", do not retag/push the images, only resolve the digests of
# the already-pushed target tags and upload them for signing. Use it to re-run the signing step of a
# previous build (e.g. after a transient signing failure) without repeating the retag. The images must
# already exist in the registry, otherwise the digest resolution fails loudly. Has no effect on dry
# runs (nothing is signed then). Set by the "Skip retag" input field on UI builds.
SKIP_RETAG=$(buildkite-agent meta-data get SKIP_RETAG --default="false")

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

# SIGN_DISTRIBUTION_IMAGES (default false): also sign the stack-version tags pushed to the original
# DOCKER_IMAGE repo (package-registry/distribution), not only DOCKER_IMAGE_RENAMED. A cosign signature
# is stored per repository (keyed by digest) and does not follow the retag, so the renamed repo is
# always signed here; the original distribution repo is assumed to be signed upstream by
# package-storage-infra unless this flag opts into signing it too.
SIGN_DISTRIBUTION_IMAGES="${SIGN_DISTRIBUTION_IMAGES:-false}"

# Target images whose stack-version tags must be signed, collected as they are pushed.
SIGNING_IMAGES=()

echo "Docker retag"
setup_buildx

for image in "${DOCKER_IMAGE_TARGETS[@]}"; do
    # Decide once per repo whether its pushed tags are collected for signing: the renamed repo
    # always, the original distribution repo only when SIGN_DISTRIBUTION_IMAGES is set.
    sign_repo=false
    if [[ "${image}" == "${DOCKER_IMAGE_RENAMED:-}" ]] \
        || [[ "${SIGN_DISTRIBUTION_IMAGES}" == "true" && "${image}" == "${DOCKER_IMAGE}" ]]; then
        sign_repo=true
    fi

    # NOTE: the non-slim tags (lite*, production*) may be deprecated in the future.
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
            # SKIP_RETAG reuses the image pushed by a previous run; otherwise retag it now.
            if [[ "${SKIP_RETAG}" == "true" ]]; then
                echo "SKIP_RETAG: not retagging, will sign existing image: ${target_image}"
            else
                retry 3 docker buildx imagetools create -t "${target_image}" "${source_image}"
                echo "Docker image pushed: ${target_image}"
            fi
            if [[ "${sign_repo}" == "true" ]]; then
                SIGNING_IMAGES+=("${target_image}")
            fi
        fi
    done
done

# Upload the digests of the pushed images for the unified-release-container-image-signing pipeline,
# one artifact per distribution. crane reads the digests from the registry, so it is only installed
# and used when there are images to sign (pushed in this run, or in a previous one under SKIP_RETAG).
if [[ ${#SIGNING_IMAGES[@]} -gt 0 ]]; then
    with_crane

    DIGEST_FILE="package-registry-distribution-${TAG_NAME}-images-digest.txt"
    : > "${DIGEST_FILE}"
    for target_image in "${SIGNING_IMAGES[@]}"; do
        write_image_digest_file "${target_image}" "${DIGEST_FILE}.tmp"
        cat "${DIGEST_FILE}.tmp" >> "${DIGEST_FILE}"
    done
    rm -f "${DIGEST_FILE}.tmp"
    retry 5 buildkite-agent artifact upload "${DIGEST_FILE}"
fi

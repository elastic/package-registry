#!/bin/bash
# Helpers for publish.sh. This file is meant to be sourced, it has no side effects.
# Requires the "retry" function from tooling.sh.

# Set the namespace and the tags of the images to be pushed, from the Buildkite environment:
# - pull requests are pushed to DOCKER_IMG_PR, using the tag PR-<number>
# - other builds are pushed to DOCKER_IMG, using the branch name as tag
# - if there is a git tag, it is used instead as tag
# Sets DOCKER_NAMESPACE, TAG_NAME, DOCKER_IMG_TAG (commit) and DOCKER_IMG_TAG_BRANCH (rolling tag).
set_image_tags() {
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
}

# Build and push the multi-platform image of a flavour. Requires set_image_tags to be called first.
#
# Usage: build_push_docker_image <runner_image> [<tag_suffix>]
build_push_docker_image() {
	local runner_image="${1}"
	local tag_suffix="${2:-}"
	local fips=0
	if [[ "${tag_suffix}" == "-fips" ]]; then
		fips=1
	fi
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
		--build-arg FIPS="${fips}" \
		--label BRANCH_NAME="${TAG_NAME}" \
		--label GIT_SHA="${BUILDKITE_COMMIT}" \
		--label GO_VERSION="${SETUP_GOLANG_VERSION}" \
		--label TIMESTAMP="$(date +%Y-%m-%d_%H:%M)" \
		.

	echo "Docker images pushed: ${docker_img_tag} ${docker_img_tag_branch}"
}

# Resolve the digest of an image already pushed to the registry and write
# "<image_ref>@<digest>" to the given file. These files are the input of the
# unified-release-container-image-signing pipeline.
#
# Usage: write_image_digest_file <image_ref> <digest_file>
write_image_digest_file() {
	local image_ref="$1"
	local digest_file="$2"
	local digest

	if ! command -v crane > /dev/null 2>&1; then
		echo "ERROR: crane command not found" >&2
		return 1
	fi

	if ! digest="$(retry 5 crane digest "${image_ref}")" \
		|| [[ -z "${digest}" || "${digest}" == "null" ]]; then
		echo "ERROR: could not resolve digest for ${image_ref}" >&2
		return 1
	fi

	echo "${image_ref}@${digest}" > "${digest_file}"
	echo "Wrote image digest for signing: ${image_ref}@${digest} -> ${digest_file}"
}

# Resolve the digest of a flavour just pushed to the registry and upload its digest file as a
# Buildkite artifact, so the unified-release-container-image-signing pipeline can collect it with
# the "package-registry-*-images-digest.txt" glob. Requires set_image_tags to have run and crane
# to be installed. The rolling/release tag and the commit tag share one digest, so a single
# reference per flavour is reported (using the rolling/release tag).
#
# Usage: upload_image_digest_for_signing <flavour> [<tag_suffix>]
upload_image_digest_for_signing() {
	local flavour="$1"
	local tag_suffix="${2:-}"
	local image_ref="${DOCKER_IMG_TAG_BRANCH}${tag_suffix}"
	local digest_file="package-registry-${flavour}-images-digest.txt"

	write_image_digest_file "${image_ref}" "${digest_file}"
	retry 5 buildkite-agent artifact upload "${digest_file}"
}

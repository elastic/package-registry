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

#!/bin/bash
# Helpers for publish.sh. This file is meant to be sourced, it has no side effects.
# Requires the "retry" function from tooling.sh.

# Set the namespace and the tags of the images to be pushed, from the Buildkite environment:
# - pull requests are pushed to DOCKER_IMG_PR, using the tag PR-<number>
# - other builds are pushed to DOCKER_IMG, using the branch name as tag
# - if there is a git tag, it is used instead as tag
# Sets DOCKER_NAMESPACE, TAG_NAME, DOCKER_IMG_TAG (commit), DOCKER_IMG_TAG_BRANCH (rolling/release
# tag) and SIGN_IMAGE_REF (the reference the signing step resolves for this build).
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

	# Reference the signing step resolves to find this build's image, chosen so a concurrent publish
	# cannot make it read another build's digest: the commit tag on branch builds (unique among
	# concurrent branch builds), the release tag on tag builds (pushed only by the tag build, and the
	# image actually released). Signing is by digest, so this only has to resolve to the right image.
	if [ -n "${BUILDKITE_TAG:-}" ]; then
		SIGN_IMAGE_REF="${DOCKER_IMG_TAG_BRANCH}"
	else
		SIGN_IMAGE_REF="${DOCKER_IMG_TAG}"
	fi
}

# Create a buildx builder and select it as the current one. Call once per publish run, before the
# image build/push or retag calls: a single builder then serves every flavour, instead of one builder
# being created per build.
setup_buildx() {
	docker buildx create --use
}

# Build and push the multi-platform image of a flavour. Requires set_image_tags and setup_buildx to
# be called first.
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
	local go_version
	go_version=$(cat .go-version)

	# Branch and PR builds push the commit tag alongside the branch/PR tag; tag builds push only the
	# release tag (the commit image already exists from the branch build, and re-pushing it would
	# overwrite it with a different rebuilt digest). See set_image_tags for how signing picks a ref.
	local push_tags=("${docker_img_tag_branch}")
	if [[ -z "${BUILDKITE_TAG:-}" ]]; then
		push_tags=("${docker_img_tag}" "${docker_img_tag_branch}")
	fi

	local tag_args=() tag
	for tag in "${push_tags[@]}"; do
		tag_args+=(-t "${tag}")
	done

	retry 3 docker buildx build --push \
		--platform linux/amd64,linux/arm64/v8 \
		--progress plain \
		"${tag_args[@]}" \
		--build-arg GO_VERSION="${go_version}" \
		--build-arg BUILDER_IMAGE=docker.elastic.co/wolfi/go \
		--build-arg RUNNER_IMAGE="${runner_image}" \
		--build-arg FIPS="${fips}" \
		--label BRANCH_NAME="${TAG_NAME}" \
		--label GIT_SHA="${BUILDKITE_COMMIT}" \
		--label GO_VERSION="${go_version}" \
		--label TIMESTAMP="$(date +%Y-%m-%d_%H:%M)" \
		.

	echo "+++ Docker images pushed: ${push_tags[*]}"
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
# the "package-registry-*-images-digest.txt" glob. Requires set_image_tags to have run (it picks
# SIGN_IMAGE_REF) and crane to be installed. Signing is by digest; the branch/release name reaches
# signing separately as the IMAGES_TAG annotation in .buildkite/pipeline.yml.
#
# Usage: upload_image_digest_for_signing <flavour> [<tag_suffix>]
upload_image_digest_for_signing() {
	local flavour="$1"
	local tag_suffix="${2:-}"
	local image_ref="${SIGN_IMAGE_REF}${tag_suffix}"
	local digest_file="package-registry-${flavour}-images-digest.txt"

	# PR builds upload digest files only to exercise this path end to end; they must never be collected
	# by the signing pipeline. Prefix the filename so it does not match ARTIFACT_PATTERN
	# ("package-registry-*-images-digest.txt") in .buildkite/pipeline.yml. The signing trigger steps are
	# also gated off for PRs by their "if:" conditions; this naming is a second, independent safety net.
	if [[ "${BUILDKITE_PULL_REQUEST}" != "false" ]]; then
		digest_file="pr-${digest_file}"
	fi

	write_image_digest_file "${image_ref}" "${digest_file}"
	retry 5 buildkite-agent artifact upload "${digest_file}"
}

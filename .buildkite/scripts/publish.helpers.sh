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
# build_push_docker_image calls: a single builder then serves every flavour, instead of one builder
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

# Write the build-unique reference of a flavour to its ref file (a single line, frozen in the uploaded
# artifact so a retry of the digest step reuses the exact image this build pushed) and upload it as a
# Buildkite artifact, so the separate digest step (resolve_image_digests.sh) can resolve its digest
# after the push. Decoupling resolution from the push keeps a transient crane/registry failure from
# failing an already-successful push (which would leave the pushed image unsigned), and forwards a
# build-unique ref so the digest step resolves the exact image this build pushed, not whatever a shared
# tag points at. Requires set_image_tags to have run (it picks SIGN_IMAGE_REF).
#
# Usage: upload_image_ref_for_signing <flavour> [<tag_suffix>]
upload_image_ref_for_signing() {
	local flavour="$1"
	local tag_suffix="${2:-}"
	local image_ref="${SIGN_IMAGE_REF}${tag_suffix}"
	local ref_file="image-ref-${flavour}.txt"

	echo "${image_ref}" > "${ref_file}"
	echo "Wrote image ref for signing: ${image_ref} -> ${ref_file}"
	retry 5 buildkite-agent artifact upload "${ref_file}"
}

# Print the digest artifact filename for a flavour. Single source of truth for the name the signing
# pipeline collects: it must stay in sync with the ARTIFACT_PATTERN
# ("package-registry-*-images-digest.txt") in .buildkite/pipeline.yml. On PR builds the name is
# prefixed with "pr-" so it does NOT match that pattern: PR builds resolve digests to exercise the
# path, but their files must never be collected by the signing pipeline (which is also gated off for
# PRs by the trigger steps' "if:" conditions).
#
# Usage: digest_file_name <flavour>
digest_file_name() {
	local flavour="$1"
	local digest_file="package-registry-${flavour}-images-digest.txt"

	if [[ "${BUILDKITE_PULL_REQUEST}" != "false" ]]; then
		digest_file="pr-${digest_file}"
	fi
	echo "${digest_file}"
}

# Resolve a signing digest for every image-ref-*.txt in the current directory (uploaded by the publish
# step and downloaded by the digest step), writing and uploading the matching digest file. Requires
# crane to be installed. Signing is by digest; the branch/release name reaches signing separately as
# the IMAGES_TAG annotation in .buildkite/pipeline.yml. A ref that cannot be resolved does not abort
# the others (every resolvable image is still signed) but makes the function return non-zero, so the
# step fails and can be retriggered without re-running the push.
resolve_image_digests() {
	local ref_file flavour image_ref digest_file
	local found=0
	local failed=0

	for ref_file in image-ref-*.txt; do
		# With no matches the glob stays literal, so skip the non-existent file.
		[[ -e "${ref_file}" ]] || continue
		found=1

		flavour="${ref_file#image-ref-}"
		flavour="${flavour%.txt}"
		read -r image_ref < "${ref_file}"
		digest_file="$(digest_file_name "${flavour}")"

		# write_image_digest_file already logs the failure; just record it so the other refs still
		# resolve and the step ends non-zero (retriggerable).
		if write_image_digest_file "${image_ref}" "${digest_file}"; then
			retry 5 buildkite-agent artifact upload "${digest_file}"
		else
			failed=1
		fi
	done

	if [[ "${found}" -eq 0 ]]; then
		echo "No image-ref-*.txt artifacts found; nothing to resolve"
	fi

	return "${failed}"
}

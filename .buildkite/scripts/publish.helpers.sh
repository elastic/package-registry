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

# Build and push the multi-platform image of a flavour, writing the buildx result metadata (which
# carries the pushed multi-arch index digest) to buildx-metadata-<flavour>.json. Requires
# set_image_tags and setup_buildx to be called first.
#
# Usage: build_push_docker_image <runner_image> [<tag_suffix>]
build_push_docker_image() {
	local runner_image="${1}"
	local tag_suffix="${2:-}"
	local fips=0
	if [[ "${tag_suffix}" == "-fips" ]]; then
		fips=1
	fi
	# flavour derived from the suffix ("" -> base, "-ubi" -> ubi, "-fips" -> fips), used to name the
	# buildx metadata file so upload_image_digest_for_signing can find this flavour's digest.
	local flavour="${tag_suffix#-}"
	flavour="${flavour:-base}"
	local metadata_file="buildx-metadata-${flavour}.json"
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

	# buildx writes the result metadata (incl. the pushed multi-arch index digest) only on a successful
	# build, so remove any stale file from a previous attempt before building.
	rm -f "${metadata_file}"

	retry 3 docker buildx build --push \
		--platform linux/amd64,linux/arm64/v8 \
		--progress plain \
		"${tag_args[@]}" \
		--metadata-file "${metadata_file}" \
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

# Print the pushed multi-arch index digest recorded in a buildx --metadata-file. This is resolved at
# push time (immutable, no registry read-back), so it is immune to a concurrent build repointing a
# shared tag. Guards that the metadata describes an image INDEX (manifest list) and that the digest is
# present, failing loudly otherwise. Requires jq.
#
# Usage: buildx_metadata_digest <metadata_file>
buildx_metadata_digest() {
	local metadata_file="$1"
	local mediatype digest

	if [[ ! -f "${metadata_file}" ]]; then
		echo "ERROR: buildx metadata file not found: ${metadata_file}" >&2
		return 1
	fi

	# A multi-platform --push produces an image index; cosign --recursive signs that index digest, not
	# a per-platform manifest digest, so assert the metadata actually describes an index.
	mediatype="$(jq -r '.["containerimage.descriptor"].mediaType // ""' "${metadata_file}")"
	case "${mediatype}" in
	*image.index.v1+json | *manifest.list.v2+json) ;;
	*)
		echo "ERROR: expected a multi-arch image index in ${metadata_file}, got mediaType '${mediatype}'" >&2
		return 1
		;;
	esac

	digest="$(jq -r '.["containerimage.digest"] // ""' "${metadata_file}")"
	if [[ -z "${digest}" || "${digest}" == "null" ]]; then
		echo "ERROR: no containerimage.digest in ${metadata_file}" >&2
		return 1
	fi

	echo "${digest}"
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

# Write the digest file for a flavour from its buildx metadata and upload it as a Buildkite artifact,
# so the unified-release-container-image-signing pipeline can collect it with the ARTIFACT_PATTERN
# glob. Requires set_image_tags (it picks SIGN_IMAGE_REF) and the flavour's build_push_docker_image
# (it writes buildx-metadata-<flavour>.json) to have run. The digest comes from the buildx metadata,
# so it is this build's exact pushed image regardless of any later tag repoint; the branch/release name
# reaches signing separately as the IMAGES_TAG annotation in .buildkite/pipeline.yml.
#
# Usage: upload_image_digest_for_signing <flavour> [<tag_suffix>]
upload_image_digest_for_signing() {
	local flavour="$1"
	local tag_suffix="${2:-}"
	local metadata_file="buildx-metadata-${flavour}.json"
	local image_ref="${SIGN_IMAGE_REF}${tag_suffix}"
	local digest_file digest
	digest_file="$(digest_file_name "${flavour}")"
	digest="$(buildx_metadata_digest "${metadata_file}")" || return 1

	echo "${image_ref}@${digest}" > "${digest_file}"
	echo "Wrote image digest for signing: ${image_ref}@${digest} -> ${digest_file}"
	retry 5 buildkite-agent artifact upload "${digest_file}"
}

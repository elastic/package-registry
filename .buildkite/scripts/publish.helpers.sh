#!/bin/bash
# Helpers for publish.sh. This file is meant to be sourced, it has no side effects.
# Requires the "retry" function from tooling.sh.

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

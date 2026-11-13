#!/bin/bash

# Helpers specific to publish-distribution.sh, which retags already-built, already-signed
# distribution images with "docker buildx imagetools create" (it does not build them).
#
# Unlike the base package-registry publish path (publish.helpers.sh), which captures the pushed
# digest from the "docker buildx build --metadata-file" result, the retag path resolves the digest
# with "crane digest" read-back from the registry. "imagetools create" does not emit the
# containerimage.descriptor metadata that buildx_metadata_digest asserts on, and crane read-back is
# the way the other Elastic pipelines feeding unified-release-container-image-signing (e.g.
# cloud-on-k8s, secret-service) resolve the signing digest. These helpers are kept here, local to the
# distribution path, so the shared publish.helpers.sh stays crane-free.

# CRANE_VERSION is set by the pipeline env (.buildkite/release-package-registry-distribution.yml); the
# default keeps with_crane usable when sourced outside that pipeline (e.g. local runs).
CRANE_VERSION="${CRANE_VERSION:-v0.22.1}"

# Install crane into ${WORKSPACE}/bin and add it to PATH. Relies on the platform helpers
# (create_bin_folder, check_platform_architecture, add_bin_path, platform_type/arch_type) sourced
# from pre-install-command.sh.
with_crane() {
    create_bin_folder
    check_platform_architecture

    echo "--- Install crane ${CRANE_VERSION}"
    # crane release assets use the raw OS name (Linux/Darwin) and x86_64/arm64
    local crane_arch="${arch_type}"
    if [[ "${arch_type}" == "amd64" ]]; then
        crane_arch="x86_64"
    fi
    local binary="go-containerregistry_${platform_type}_${crane_arch}.tar.gz"
    local tmp_folder="${WORKSPACE}/tmp"
    mkdir -p "${tmp_folder}"
    local tar_filepath="${tmp_folder}/crane.tar.gz"

    # --fail makes curl exit non-zero on HTTP errors (e.g. 429/503) so the retry loop
    # catches them, instead of saving the error body and letting tar fail on it.
    retry 5 curl --fail -sSL -o "${tar_filepath}" "https://github.com/google/go-containerregistry/releases/download/${CRANE_VERSION}/${binary}"
    tar -C "${WORKSPACE}/bin" -xpf "${tar_filepath}" crane
    chmod +x "${WORKSPACE}/bin/crane"
    rm -rf "${tmp_folder}"

    add_bin_path
    crane version
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

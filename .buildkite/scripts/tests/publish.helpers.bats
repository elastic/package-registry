#!/usr/bin/env bats

# bats file_tags=tag:publish-helpers

setup() {
    load 'setup_command'
    _common_setup
    _set_publish_env

    sources="source .buildkite/scripts/tooling.sh; source .buildkite/scripts/publish.helpers.sh;"
}

# Build tests run build_push_docker_image in the repo root (it reads .go-version); the docker mock
# writes a buildx metadata file there per flavour. Remove those so they do not linger.
teardown() {
    rm -f buildx-metadata-*.json
}

# Write the command file that calls set_image_tags and prints the resulting variables, then run it.
_run_set_image_tags() {
    cat <<EOF > "${COMMAND_FILE}"
${sources}
set_image_tags
echo "\${DOCKER_NAMESPACE} \${TAG_NAME} \${DOCKER_IMG_TAG} \${DOCKER_IMG_TAG_BRANCH}"
EOF
    run ${test_command}
}

# Write the command file that builds a flavour (set_image_tags + build_push_docker_image), then run.
#
# Usage: _run_build_push <runner_image> [<tag_suffix>]
_run_build_push() {
    cat <<EOF > "${COMMAND_FILE}"
${sources}
set_image_tags
build_push_docker_image "$1" "${2:-}"
EOF
    run ${test_command}
}

# Write the command file that calls setup_buildx, then run it.
_run_setup_buildx() {
    cat <<EOF > "${COMMAND_FILE}"
${sources}
setup_buildx
EOF
    run ${test_command}
}

# Print the "docker buildx build" call recorded under test. Each test here builds a single flavour,
# so the one buildx build line is unambiguous.
_build_call() {
    grep -F -- "buildx build --push" "${DOCKER_CALLS_FILE}"
}

# Write the command file that prints digest_file_name for a flavour, then run it.
#
# Usage: _run_digest_file_name <flavour>
_run_digest_file_name() {
    cat <<EOF > "${COMMAND_FILE}"
${sources}
digest_file_name "$1"
EOF
    run ${test_command}
}

# Write the command file that writes + uploads the digest file of a flavour from a prepared buildx
# metadata file, run from a scratch dir so the digest files do not land in the repository. Prepare
# buildx-metadata-<flavour>.json in UPLOAD_WORKDIR (set by the test) before calling.
#
# Usage: _run_upload_digest <flavour> [<tag_suffix>]
_run_upload_digest() {
    cat <<EOF > "${COMMAND_FILE}"
cd "${UPLOAD_WORKDIR}"
source ${ROOT_DIR}/.buildkite/scripts/tooling.sh
source ${ROOT_DIR}/.buildkite/scripts/publish.helpers.sh
set_image_tags
upload_image_digest_for_signing "$1" "${2:-}"
EOF
    run ${test_command}
}

@test "set_image_tags uses the branch name for branch builds" {
    _set_buildkite_env main false
    _run_set_image_tags
    assert_success
    assert_output "${DOCKER_IMG} main ${DOCKER_IMG}:abc123 ${DOCKER_IMG}:main"
}

@test "set_image_tags uses the PR namespace and tag for pull requests" {
    _set_buildkite_env feature 123
    _run_set_image_tags
    assert_success
    assert_output "${DOCKER_IMG_PR} PR-123 ${DOCKER_IMG_PR}:abc123 ${DOCKER_IMG_PR}:PR-123"
}

@test "set_image_tags uses the git tag when building a tag" {
    _set_buildkite_env main false v1.2.3
    _run_set_image_tags
    assert_success
    assert_output "${DOCKER_IMG} v1.2.3 ${DOCKER_IMG}:abc123 ${DOCKER_IMG}:v1.2.3"
}

@test "docker fails loudly when a test calls it without _mock_docker" {
    run docker images
    assert_failure
    assert_output --partial "ERROR: 'docker' called in a bats test without _mock_docker"
}

@test "setup_buildx creates and selects a single buildx builder" {
    _mock_docker
    _run_setup_buildx
    assert_success
    run grep -c "^buildx create --use" "${DOCKER_CALLS_FILE}"
    assert_output "1"
}

@test "build_push_docker_image builds the base flavour for amd64 and arm64" {
    _set_buildkite_env main false
    _mock_docker
    _run_build_push "docker.elastic.co/wolfi/chainguard-base" ""
    assert_success
    assert_output --partial "Docker images pushed: ${DOCKER_IMG}:abc123 ${DOCKER_IMG}:main"
    # build_push_docker_image no longer creates a builder; setup_buildx does that once per run.
    run grep -c "^buildx create --use" "${DOCKER_CALLS_FILE}"
    assert_output "0"
    run _build_call
    assert_output --partial "buildx build --push --platform linux/amd64,linux/arm64/v8 --progress plain"
    assert_output --partial "-t ${DOCKER_IMG}:abc123 -t ${DOCKER_IMG}:main --metadata-file buildx-metadata-base.json --build-arg"
    assert_output --partial "--build-arg GO_VERSION=$(cat .go-version)"
    assert_output --partial "--build-arg BUILDER_IMAGE=docker.elastic.co/wolfi/go"
    assert_output --partial "--build-arg RUNNER_IMAGE=docker.elastic.co/wolfi/chainguard-base --build-arg FIPS=0"
    assert_output --partial "--label BRANCH_NAME=main --label GIT_SHA=abc123 --label GO_VERSION=$(cat .go-version) --label TIMESTAMP="
}

@test "build_push_docker_image adds the suffix to the tags of the ubi flavour" {
    _set_buildkite_env main false
    _mock_docker
    _run_build_push "registry.access.redhat.com/ubi9/ubi-minimal:9.6" "-ubi"
    assert_success
    run _build_call
    assert_output --partial "-t ${DOCKER_IMG}:abc123-ubi -t ${DOCKER_IMG}:main-ubi --metadata-file buildx-metadata-ubi.json --build-arg"
    assert_output --partial "--build-arg RUNNER_IMAGE=registry.access.redhat.com/ubi9/ubi-minimal:9.6 --build-arg FIPS=0"
}

@test "build_push_docker_image enables FIPS in the fips flavour" {
    _set_buildkite_env main false
    _mock_docker
    _run_build_push "docker.elastic.co/wolfi/chainguard-base-fips" "-fips"
    assert_success
    run _build_call
    assert_output --partial "--metadata-file buildx-metadata-fips.json"
    assert_output --partial "--build-arg RUNNER_IMAGE=docker.elastic.co/wolfi/chainguard-base-fips --build-arg FIPS=1"
}

@test "build_push_docker_image pushes only the git tag when building a tag" {
    _set_buildkite_env main false v1.2.3
    _mock_docker
    _run_build_push "docker.elastic.co/wolfi/chainguard-base" ""
    assert_success
    run _build_call
    assert_output --partial "-t ${DOCKER_IMG}:v1.2.3 --metadata-file buildx-metadata-base.json --build-arg"
    assert_output --partial "--label BRANCH_NAME=v1.2.3"
    # the commit tag is not re-pushed on tag builds (it already exists from the branch build)
    refute_output --partial "${DOCKER_IMG}:abc123"
}

@test "build_push_docker_image retries a failed build" {
    _set_buildkite_env main false
    _mock_docker
    export DOCKER_STUB_BUILD_FAILURES=2
    _run_build_push "docker.elastic.co/wolfi/chainguard-base" ""
    assert_success
    run grep -c "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_output "3"
}

@test "build_push_docker_image fails when the build keeps failing" {
    _set_buildkite_env main false
    _mock_docker
    export DOCKER_STUB_BUILD_FAILURES=100
    cat <<EOF > "${COMMAND_FILE}"
${sources}
set_image_tags
build_push_docker_image "docker.elastic.co/wolfi/chainguard-base" ""
echo "must not be reached"
EOF

    run ${test_command}
    assert_failure 1
    refute_output --partial "must not be reached"
    refute_output --partial "Docker images pushed"
    run grep -c "^buildx build" "${DOCKER_CALLS_FILE}"
    assert_output "3"
}

@test "buildx_metadata_digest extracts the multi-arch index digest" {
    cat > "${BATS_TEST_TMPDIR}/meta.json" <<'JSON'
{"containerimage.digest":"sha256:feedface","containerimage.descriptor":{"mediaType":"application/vnd.oci.image.index.v1+json","digest":"sha256:feedface"}}
JSON
    cat <<EOF > "${COMMAND_FILE}"
${sources}
buildx_metadata_digest "${BATS_TEST_TMPDIR}/meta.json"
EOF
    run ${test_command}
    assert_success
    assert_output "sha256:feedface"
}

@test "buildx_metadata_digest fails when the metadata file is missing" {
    cat <<EOF > "${COMMAND_FILE}"
${sources}
buildx_metadata_digest "${BATS_TEST_TMPDIR}/nope.json"
EOF
    run ${test_command}
    assert_failure
    assert_output --partial "buildx metadata file not found"
}

@test "buildx_metadata_digest fails when the digest is absent" {
    echo '{"containerimage.descriptor":{"mediaType":"application/vnd.oci.image.index.v1+json"}}' > "${BATS_TEST_TMPDIR}/meta.json"
    cat <<EOF > "${COMMAND_FILE}"
${sources}
buildx_metadata_digest "${BATS_TEST_TMPDIR}/meta.json"
EOF
    run ${test_command}
    assert_failure
    assert_output --partial "no containerimage.digest"
}

# A single-platform build produces a manifest, not an index; signing needs the index digest, so this
# must be rejected rather than silently signing a per-platform manifest.
@test "buildx_metadata_digest rejects a non-index (single-platform) manifest" {
    echo '{"containerimage.digest":"sha256:feedface","containerimage.descriptor":{"mediaType":"application/vnd.oci.image.manifest.v1+json"}}' > "${BATS_TEST_TMPDIR}/meta.json"
    cat <<EOF > "${COMMAND_FILE}"
${sources}
buildx_metadata_digest "${BATS_TEST_TMPDIR}/meta.json"
EOF
    run ${test_command}
    assert_failure
    assert_output --partial "expected a multi-arch image index"
}

@test "digest_file_name returns the signing name on non-PR builds" {
    _set_buildkite_env main false
    _run_digest_file_name "base"
    assert_success
    assert_output "package-registry-base-images-digest.txt"
}

# PR builds resolve digests to exercise the path, but their files must not be collected by the signing
# pipeline, so the name is prefixed with pr- (which does not match ARTIFACT_PATTERN).
@test "digest_file_name prefixes the name with pr- on PR builds" {
    _set_buildkite_env feature 123
    _run_digest_file_name "base"
    assert_success
    assert_output "pr-package-registry-base-images-digest.txt"
}

@test "upload_image_digest_for_signing writes and uploads the flavour digest file" {
    _set_buildkite_env main false
    _mock_buildkite_agent
    UPLOAD_WORKDIR="${BATS_TEST_TMPDIR}/work"
    mkdir -p "${UPLOAD_WORKDIR}"
    echo '{"containerimage.digest":"sha256:feedface","containerimage.descriptor":{"mediaType":"application/vnd.oci.image.index.v1+json"}}' > "${UPLOAD_WORKDIR}/buildx-metadata-base.json"
    _run_upload_digest "base" ""
    assert_success
    assert_equal "$(cat "${UPLOAD_WORKDIR}/package-registry-base-images-digest.txt")" "${DOCKER_IMG}:abc123@sha256:feedface"
    assert_equal "$(cat "${BUILDKITE_AGENT_CALLS_FILE}")" "artifact upload package-registry-base-images-digest.txt"
}

# Tag builds sign by the release tag (:<tag>); branch builds sign by the commit tag.
@test "upload_image_digest_for_signing uses the git tag and the flavour suffix on tag builds" {
    _set_buildkite_env main false v1.2.3
    _mock_buildkite_agent
    UPLOAD_WORKDIR="${BATS_TEST_TMPDIR}/work"
    mkdir -p "${UPLOAD_WORKDIR}"
    echo '{"containerimage.digest":"sha256:feedface","containerimage.descriptor":{"mediaType":"application/vnd.oci.image.index.v1+json"}}' > "${UPLOAD_WORKDIR}/buildx-metadata-fips.json"
    _run_upload_digest "fips" "-fips"
    assert_success
    assert_equal "$(cat "${UPLOAD_WORKDIR}/package-registry-fips-images-digest.txt")" "${DOCKER_IMG}:v1.2.3-fips@sha256:feedface"
}

@test "upload_image_digest_for_signing writes PR digest files under the non-collectable pr- name" {
    _set_buildkite_env feature 123
    _mock_buildkite_agent
    UPLOAD_WORKDIR="${BATS_TEST_TMPDIR}/work"
    mkdir -p "${UPLOAD_WORKDIR}"
    echo '{"containerimage.digest":"sha256:feedface","containerimage.descriptor":{"mediaType":"application/vnd.oci.image.index.v1+json"}}' > "${UPLOAD_WORKDIR}/buildx-metadata-base.json"
    _run_upload_digest "base" ""
    assert_success
    assert_equal "$(cat "${UPLOAD_WORKDIR}/pr-package-registry-base-images-digest.txt")" "${DOCKER_IMG_PR}:abc123@sha256:feedface"
    assert_file_not_exists "${UPLOAD_WORKDIR}/package-registry-base-images-digest.txt"
}

# The signing trigger step in .buildkite/pipeline.yml collects the digest files with a glob
# (ARTIFACT_PATTERN). This checks that digest_file_name on a non-PR build matches that glob and that the
# pr- prefixed PR name does NOT, so the two sides cannot drift apart silently.
@test "the signing ARTIFACT_PATTERN matches non-PR digest names but not the pr- names" {
    local pattern
    pattern="$(grep -m1 'ARTIFACT_PATTERN:' .buildkite/pipeline.yml | sed -E 's/.*"([^"]*)".*/\1/')"
    refute [ -z "${pattern}" ]

    _set_buildkite_env main false
    _run_digest_file_name "base"
    assert_success
    case "${output}" in
        ${pattern}) ;;
        *) fail "pipeline ARTIFACT_PATTERN '${pattern}' does not match non-PR digest name '${output}'" ;;
    esac

    _set_buildkite_env feature 123
    _run_digest_file_name "base"
    assert_success
    case "${output}" in
        ${pattern}) fail "PR digest name '${output}' must not match signing ARTIFACT_PATTERN '${pattern}'" ;;
        *) ;;
    esac
}

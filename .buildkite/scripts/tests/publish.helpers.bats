#!/usr/bin/env bats

# bats file_tags=tag:publish-helpers

setup() {
    load 'setup_command'
    _common_setup
    _set_publish_env

    sources="source .buildkite/scripts/tooling.sh; source .buildkite/scripts/publish.helpers.sh;"
    DIGEST_FILE="${BATS_TEST_TMPDIR}/package-registry-images-digest.txt"
    IMAGE_REF="docker.elastic.co/package-registry/package-registry:main"
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

# Write the command file that resolves the digest of IMAGE_REF into DIGEST_FILE, then run it.
_run_write_digest() {
    cat <<EOF > "${COMMAND_FILE}"
${sources}
write_image_digest_file "${IMAGE_REF}" "${DIGEST_FILE}"
EOF
    run ${test_command}
}

# Setup only needed by the tests that resolve digests: replace crane with a stub.
#   CRANE_STUB_DIGEST: digest printed by "crane digest" (empty prints nothing)
#   CRANE_STUB_FAILURES: number of calls that fail before "crane digest" succeeds
_mock_crane() {
    export CRANE_STUB_DIGEST="${1:-}"
    export CRANE_STUB_FAILURES="${2:-0}"
    export CRANE_CALLS_FILE="${BATS_TEST_TMPDIR}/crane_calls"
    echo 0 > "${CRANE_CALLS_FILE}"

    _create_stub crane <<'EOF'
calls="$(cat "${CRANE_CALLS_FILE}")"
echo $((calls + 1)) > "${CRANE_CALLS_FILE}"
echo "$*" >> "${CRANE_CALLS_FILE}.args"
if [[ "${calls}" -lt "${CRANE_STUB_FAILURES}" ]]; then
    exit 1
fi
echo "${CRANE_STUB_DIGEST}"
EOF
}

# Print the "docker buildx build" call recorded under test. Each test here builds a single flavour,
# so the one buildx build line is unambiguous.
_build_call() {
    grep -F -- "buildx build --push" "${DOCKER_CALLS_FILE}"
}

# Write the command file that writes and uploads the build-unique ref of a flavour, run from a scratch
# dir so the ref files do not land in the repository. Sets UPLOAD_WORKDIR to that scratch dir.
#
# Usage: _run_upload_ref <flavour> [<tag_suffix>]
_run_upload_ref() {
    UPLOAD_WORKDIR="${BATS_TEST_TMPDIR}/work"
    mkdir -p "${UPLOAD_WORKDIR}"
    cat <<EOF > "${COMMAND_FILE}"
cd "${UPLOAD_WORKDIR}"
source ${ROOT_DIR}/.buildkite/scripts/tooling.sh
source ${ROOT_DIR}/.buildkite/scripts/publish.helpers.sh
set_image_tags
upload_image_ref_for_signing "$1" "${2:-}"
EOF
    run ${test_command}
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

# Write the command file that runs resolve_image_digests from BATS_TEST_TMPDIR, then run it. Seed the
# image-ref-*.txt files in ${BATS_TEST_TMPDIR} (and set the crane / buildkite-agent mocks) before
# calling; the resolved digest files land in ${BATS_TEST_TMPDIR} too.
_run_resolve_image_digests() {
    cat <<EOF > "${COMMAND_FILE}"
${sources}
cd "${BATS_TEST_TMPDIR}"
resolve_image_digests
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
    assert_output --partial "-t ${DOCKER_IMG}:abc123 -t ${DOCKER_IMG}:main --build-arg"
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
    assert_output --partial "-t ${DOCKER_IMG}:abc123-ubi -t ${DOCKER_IMG}:main-ubi --build-arg"
    assert_output --partial "--build-arg RUNNER_IMAGE=registry.access.redhat.com/ubi9/ubi-minimal:9.6 --build-arg FIPS=0"
}

@test "build_push_docker_image enables FIPS in the fips flavour" {
    _set_buildkite_env main false
    _mock_docker
    _run_build_push "docker.elastic.co/wolfi/chainguard-base-fips" "-fips"
    assert_success
    run _build_call
    assert_output --partial "--build-arg RUNNER_IMAGE=docker.elastic.co/wolfi/chainguard-base-fips --build-arg FIPS=1"
}

@test "build_push_docker_image pushes only the git tag when building a tag" {
    _set_buildkite_env main false v1.2.3
    _mock_docker
    _run_build_push "docker.elastic.co/wolfi/chainguard-base" ""
    assert_success
    run _build_call
    assert_output --partial "-t ${DOCKER_IMG}:v1.2.3 --build-arg"
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

@test "write_image_digest_file writes the image reference with its digest" {
    _mock_crane "sha256:feedface"
    _run_write_digest
    assert_success
    assert_output --partial "${IMAGE_REF}@sha256:feedface"
    assert_equal "$(cat "${DIGEST_FILE}")" "${IMAGE_REF}@sha256:feedface"
    assert_equal "$(cat "${CRANE_CALLS_FILE}.args")" "digest ${IMAGE_REF}"
}

@test "write_image_digest_file overwrites an existing digest file" {
    _mock_crane "sha256:feedface"
    echo "old content" > "${DIGEST_FILE}"
    _run_write_digest
    assert_success
    assert_equal "$(cat "${DIGEST_FILE}")" "${IMAGE_REF}@sha256:feedface"
}

@test "write_image_digest_file retries when crane fails temporarily" {
    _mock_crane "sha256:feedface" 2
    _run_write_digest
    assert_success
    assert_equal "$(head -1 "${CRANE_CALLS_FILE}")" "3"
    assert_equal "$(cat "${DIGEST_FILE}")" "${IMAGE_REF}@sha256:feedface"
}

@test "write_image_digest_file fails when crane keeps failing" {
    _mock_crane "sha256:feedface" 100
    _run_write_digest
    assert_failure
    assert_output --partial "ERROR: could not resolve digest for ${IMAGE_REF}"
    assert_equal "$(head -1 "${CRANE_CALLS_FILE}")" "5"
    assert_file_not_exists "${DIGEST_FILE}"
}

@test "write_image_digest_file fails when the digest is empty" {
    _mock_crane ""
    _run_write_digest
    assert_failure
    assert_output --partial "ERROR: could not resolve digest for ${IMAGE_REF}"
    assert_file_not_exists "${DIGEST_FILE}"
}

@test "write_image_digest_file fails when the digest is null" {
    _mock_crane "null"
    _run_write_digest
    assert_failure
    assert_output --partial "ERROR: could not resolve digest for ${IMAGE_REF}"
    assert_file_not_exists "${DIGEST_FILE}"
}

@test "write_image_digest_file fails when crane is not installed" {
    # a PATH without the crane binary (the stubs are not needed, only builtins are used)
    cat <<EOF > "${COMMAND_FILE}"
${sources}
PATH="${BATS_TEST_TMPDIR}/empty"
write_image_digest_file "${IMAGE_REF}" "${DIGEST_FILE}"
EOF

    run ${test_command}
    assert_failure
    assert_output --partial "ERROR: crane command not found"
    assert_file_not_exists "${DIGEST_FILE}"
}

@test "upload_image_ref_for_signing writes and uploads the flavour ref file" {
    _set_buildkite_env main false
    _mock_buildkite_agent
    _run_upload_ref "base" ""
    assert_success
    assert_equal "$(cat "${UPLOAD_WORKDIR}/image-ref-base.txt")" "${DOCKER_IMG}:abc123"
    assert_equal "$(cat "${BUILDKITE_AGENT_CALLS_FILE}")" "artifact upload image-ref-base.txt"
}

# Tag builds forward the release tag (:<tag>) as the build-unique ref: only the tag build pushes it and
# it is the released image. Branch builds forward the commit tag instead.
@test "upload_image_ref_for_signing uses the git tag and the flavour suffix on tag builds" {
    _set_buildkite_env main false v1.2.3
    _mock_buildkite_agent
    _run_upload_ref "fips" "-fips"
    assert_success
    assert_equal "$(cat "${UPLOAD_WORKDIR}/image-ref-fips.txt")" "${DOCKER_IMG}:v1.2.3-fips"
}

@test "upload_image_ref_for_signing uses the PR namespace on PR builds" {
    _set_buildkite_env feature 123
    _mock_buildkite_agent
    _run_upload_ref "base" ""
    assert_success
    assert_equal "$(cat "${UPLOAD_WORKDIR}/image-ref-base.txt")" "${DOCKER_IMG_PR}:abc123"
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

@test "resolve_image_digests resolves each ref to its digest file and uploads it" {
    _set_buildkite_env main false
    _mock_crane "sha256:feedface"
    _mock_buildkite_agent
    echo "${DOCKER_IMG}:abc123" > "${BATS_TEST_TMPDIR}/image-ref-base.txt"
    echo "${DOCKER_IMG}:abc123-ubi" > "${BATS_TEST_TMPDIR}/image-ref-ubi.txt"
    _run_resolve_image_digests
    assert_success
    assert_equal "$(cat "${BATS_TEST_TMPDIR}/package-registry-base-images-digest.txt")" "${DOCKER_IMG}:abc123@sha256:feedface"
    assert_equal "$(cat "${BATS_TEST_TMPDIR}/package-registry-ubi-images-digest.txt")" "${DOCKER_IMG}:abc123-ubi@sha256:feedface"
    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact upload package-registry-base-images-digest.txt"
    assert_line "artifact upload package-registry-ubi-images-digest.txt"
}

# On PR builds resolve_image_digests writes the digest files under the pr- prefixed, non-collectable
# name (via digest_file_name), so the signing pipeline cannot pick up a PR image's digest.
@test "resolve_image_digests writes PR digest files under the non-collectable pr- name" {
    _set_buildkite_env feature 123
    _mock_crane "sha256:feedface"
    _mock_buildkite_agent
    echo "${DOCKER_IMG_PR}:abc123" > "${BATS_TEST_TMPDIR}/image-ref-base.txt"
    _run_resolve_image_digests
    assert_success
    assert_equal "$(cat "${BATS_TEST_TMPDIR}/pr-package-registry-base-images-digest.txt")" "${DOCKER_IMG_PR}:abc123@sha256:feedface"
    assert_file_not_exists "${BATS_TEST_TMPDIR}/package-registry-base-images-digest.txt"
}

@test "resolve_image_digests does nothing when there are no ref files" {
    _set_buildkite_env main false
    _mock_crane "sha256:feedface"
    _mock_buildkite_agent
    _run_resolve_image_digests
    assert_success
    assert_output --partial "nothing to resolve"
    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_output ""
}

# A ref that cannot be resolved does not abort the others (every resolvable image is still signed) but
# makes the step fail so it can be retriggered.
@test "resolve_image_digests fails on an unresolvable ref but still uploads the resolvable one" {
    _set_buildkite_env main false
    _mock_buildkite_agent
    # crane succeeds for the base ref and always fails for the fips ref.
    _create_stub crane <<'STUB'
if [[ "$2" == *-fips ]]; then
    exit 1
else
    echo "sha256:feedface"
fi
STUB
    echo "${DOCKER_IMG}:abc123" > "${BATS_TEST_TMPDIR}/image-ref-base.txt"
    echo "${DOCKER_IMG}:abc123-fips" > "${BATS_TEST_TMPDIR}/image-ref-fips.txt"
    _run_resolve_image_digests
    assert_failure
    assert_output --partial "could not resolve digest"
    # the resolvable image's digest file was still written and uploaded
    assert_file_exists "${BATS_TEST_TMPDIR}/package-registry-base-images-digest.txt"
    run cat "${BUILDKITE_AGENT_CALLS_FILE}"
    assert_line "artifact upload package-registry-base-images-digest.txt"
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

#!/usr/bin/env bash

_common_setup() {
    load '../../../bats_tests/test_helper/bats-support/load'
    load '../../../bats_tests/test_helper/bats-assert/load'
    load '../../../bats_tests/test_helper/bats-file/load'

    # search for the .git folder to know the workspace root
    # assume that this script is located somewhere under the git repository
    ROOT_DIR="$(git rev-parse --show-toplevel)"

    # the scripts use paths relative to the repository root
    cd "${ROOT_DIR}"

    # tests write the code to run into command.sh and run it in a new shell with the same flags
    # used in the pipelines: run ${test_command}
    # test_command is the name of a function so the path of the file (that could contain spaces
    # if TMPDIR does) is kept as a single argument
    COMMAND_FILE="${BATS_TEST_TMPDIR}/command.sh"
    test_command="_run_command_file"

    # stubs of external commands, found before the real ones
    STUB_BIN="${BATS_TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # do not wait between retries
    _create_stub sleep <<'EOF'
exit 0
EOF

    # Fail loudly if a script under test reaches the Docker daemon without the test stubbing it,
    # instead of hitting the real daemon or registry. Tests that need it override it with _mock_docker.
    _create_stub docker <<'EOF'
echo "ERROR: 'docker' called in a bats test without _mock_docker" >&2
exit 1
EOF
}

# Run COMMAND_FILE in a new shell with the same flags used in the pipelines.
_run_command_file() {
    bash -e -u -o pipefail "${COMMAND_FILE}"
}

# Create an executable stub in a directory that is first in PATH. The body is read from stdin.
#
# Usage: _create_stub <command_name> <<'EOF' ... EOF
_create_stub() {
    local name="$1"
    {
        echo '#!/bin/bash'
        cat
    } > "${STUB_BIN}/${name}"
    chmod +x "${STUB_BIN}/${name}"
}

# Environment set by the Buildkite pipeline for the publish.sh script (the BUILDKITE_* variables are
# set by each test with _set_buildkite_env).
_set_publish_env() {
    export DOCKER_IMG="docker.elastic.co/package-registry/package-registry"
    export DOCKER_IMG_PR="docker.elastic.co/observability-ci/package-registry"
}

# Variables set by the Buildkite agent for a build. They must be set explicitly by each test, the
# tests could be running in a Buildkite build and inherit its values.
#
# Usage: _set_buildkite_env <branch> <pull_request_number_or_false> [<git_tag>]
_set_buildkite_env() {
    export BUILDKITE_COMMIT="abc123"
    export BUILDKITE_BRANCH="$1"
    export BUILDKITE_PULL_REQUEST="$2"
    if [[ -n "${3:-}" ]]; then
        export BUILDKITE_TAG="$3"
    else
        unset BUILDKITE_TAG
    fi
}

# Stub of docker that records the arguments of every call in DOCKER_CALLS_FILE and fails the first
# DOCKER_STUB_BUILD_FAILURES calls of "docker buildx build". On a successful "buildx build" it emulates
# buildx writing its result metadata to the --metadata-file path (as real buildx does only on success),
# with containerimage.digest = DOCKER_STUB_DIGEST and descriptor.mediaType = DOCKER_STUB_MEDIATYPE
# (override those before calling to exercise buildx_metadata_digest's guards).
_mock_docker() {
    export DOCKER_CALLS_FILE="${BATS_TEST_TMPDIR}/docker_calls"
    export DOCKER_BUILD_FAILURES_FILE="${BATS_TEST_TMPDIR}/docker_build_failures"
    echo 0 > "${DOCKER_BUILD_FAILURES_FILE}"
    export DOCKER_STUB_DIGEST="${DOCKER_STUB_DIGEST:-sha256:deadbeef}"
    export DOCKER_STUB_MEDIATYPE="${DOCKER_STUB_MEDIATYPE:-application/vnd.oci.image.index.v1+json}"

    _create_stub docker <<'EOF'
echo "$*" >> "${DOCKER_CALLS_FILE}"
if [[ "$1 $2" == "buildx build" ]]; then
    failures="$(cat "${DOCKER_BUILD_FAILURES_FILE}")"
    if [[ "${failures}" -lt "${DOCKER_STUB_BUILD_FAILURES:-0}" ]]; then
        echo $((failures + 1)) > "${DOCKER_BUILD_FAILURES_FILE}"
        exit 1
    fi
    meta=""
    prev=""
    for arg in "$@"; do
        if [[ "${prev}" == "--metadata-file" ]]; then meta="${arg}"; fi
        prev="${arg}"
    done
    if [[ -n "${meta}" ]]; then
        printf '{"containerimage.digest":"%s","containerimage.descriptor":{"mediaType":"%s","digest":"%s","size":1}}\n' \
            "${DOCKER_STUB_DIGEST}" "${DOCKER_STUB_MEDIATYPE}" "${DOCKER_STUB_DIGEST}" > "${meta}"
    fi
fi
EOF
}

# Stub of buildkite-agent that records the arguments of every call in BUILDKITE_AGENT_CALLS_FILE.
_mock_buildkite_agent() {
    export BUILDKITE_AGENT_CALLS_FILE="${BATS_TEST_TMPDIR}/buildkite_agent_calls"
    : > "${BUILDKITE_AGENT_CALLS_FILE}"

    _create_stub buildkite-agent <<'EOF'
echo "$*" >> "${BUILDKITE_AGENT_CALLS_FILE}"
EOF
}

# Stub curl so a later with_jq call "installs" jq by copying the system jq into the download path,
# instead of fetching it from the network. Requires a working jq on PATH (the bats environment provides
# one) and WORKSPACE (with_jq installs into ${WORKSPACE}/bin). Records curl arguments in CURL_ARGS_FILE.
_mock_jq_install() {
    export CURL_ARGS_FILE="${BATS_TEST_TMPDIR}/curl_args"
    : > "${CURL_ARGS_FILE}"
    # with_jq builds the download URL from JQ_VERSION (set by the pipeline env); provide it so the
    # script does not trip over `set -u` in tests.
    export JQ_VERSION="${JQ_VERSION:-1.7}"
    REAL_JQ="$(command -v jq)"
    export REAL_JQ

    _create_stub curl <<'EOF'
echo "$*" >> "${CURL_ARGS_FILE}"
output=""
prev=""
for arg in "$@"; do
    if [[ "${prev}" == "-o" ]]; then output="${arg}"; fi
    prev="${arg}"
done
if [[ -n "${output}" ]]; then cp "${REAL_JQ}" "${output}"; fi
EOF
}

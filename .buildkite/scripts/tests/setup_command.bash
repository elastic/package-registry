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
    COMMAND_FILE="${BATS_TEST_TMPDIR}/command.sh"
    export test_command="bash -e -u -o pipefail ${COMMAND_FILE}"

    # stubs of external commands, found before the real ones
    STUB_BIN="${BATS_TEST_TMPDIR}/bin"
    mkdir -p "${STUB_BIN}"
    export PATH="${STUB_BIN}:${PATH}"

    # do not wait between retries
    _create_stub sleep <<'EOF'
exit 0
EOF

    # Fail loudly if a script under test reaches a side-effecting command without the test
    # stubbing it, instead of hitting the real Docker daemon or registry. Tests that need it
    # override it with _mock_docker.
    _create_stub docker <<'EOF'
echo "ERROR: 'docker' called in a bats test without _mock_docker" >&2
exit 1
EOF
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
    export SETUP_GOLANG_VERSION="1.2.3"
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
# DOCKER_STUB_BUILD_FAILURES calls of "docker buildx build".
_mock_docker() {
    export DOCKER_CALLS_FILE="${BATS_TEST_TMPDIR}/docker_calls"
    export DOCKER_BUILD_FAILURES_FILE="${BATS_TEST_TMPDIR}/docker_build_failures"
    echo 0 > "${DOCKER_BUILD_FAILURES_FILE}"

    _create_stub docker <<'EOF'
echo "$*" >> "${DOCKER_CALLS_FILE}"
if [[ "$1 $2" == "buildx build" ]]; then
    failures="$(cat "${DOCKER_BUILD_FAILURES_FILE}")"
    if [[ "${failures}" -lt "${DOCKER_STUB_BUILD_FAILURES:-0}" ]]; then
        echo $((failures + 1)) > "${DOCKER_BUILD_FAILURES_FILE}"
        exit 1
    fi
fi
EOF
}

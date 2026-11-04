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

    # Fail loudly if a script under test reaches a side-effecting command without the test
    # stubbing it, instead of hitting the real Docker daemon or registry. Tests that need these
    # override them with _mock_docker / _mock_crane.
    _create_stub docker <<'EOF'
echo "ERROR: 'docker' called in a bats test without _mock_docker" >&2
exit 1
EOF
    _create_stub crane <<'EOF'
echo "ERROR: 'crane' called in a bats test without _mock_crane" >&2
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

# Stub of buildkite-agent that records the arguments of every call in BUILDKITE_AGENT_CALLS_FILE.
_mock_buildkite_agent() {
    export BUILDKITE_AGENT_CALLS_FILE="${BATS_TEST_TMPDIR}/buildkite_agent_calls"
    : > "${BUILDKITE_AGENT_CALLS_FILE}"

    _create_stub buildkite-agent <<'EOF'
echo "$*" >> "${BUILDKITE_AGENT_CALLS_FILE}"
EOF
}

# Stub uname and curl so a later with_crane call installs a fake crane instead of downloading the real
# one. The fake crane answers "crane digest" with ${CRANE_STUB_DIGEST} (default sha256:fakedigest) and
# prints "fake crane ${CRANE_VERSION}" otherwise (e.g. "crane version"). Export CRANE_STUB_DIGEST or
# CRANE_STUB_VERSION before calling to override the defaults. CRANE_VERSION is forced from
# CRANE_STUB_VERSION so the pipeline-level CRANE_VERSION cannot leak into with_crane's download URL.
# uname reports <system>/<machine> (default Linux/x86_64); curl records its arguments in CURL_ARGS_FILE
# and writes the tarball to its -o path. Requires WORKSPACE (with_crane installs into ${WORKSPACE}/bin).
#
# Usage: [CRANE_STUB_DIGEST=<digest>] [CRANE_STUB_VERSION=<version>] _install_fake_crane [<system>] [<machine>]
_install_fake_crane() {
    export CRANE_STUB_DIGEST="${CRANE_STUB_DIGEST:-sha256:fakedigest}"
    export CRANE_VERSION="${CRANE_STUB_VERSION:-v0.0.1}"
    export UNAME_STUB_SYSTEM="${1:-Linux}"
    export UNAME_STUB_MACHINE="${2:-x86_64}"
    export CURL_ARGS_FILE="${BATS_TEST_TMPDIR}/curl_args"
    : > "${CURL_ARGS_FILE}"

    _create_stub uname <<'EOF'
if [[ "${1:-}" == "-m" ]]; then
    echo "${UNAME_STUB_MACHINE}"
else
    echo "${UNAME_STUB_SYSTEM}"
fi
EOF
    _create_stub curl <<'EOF'
echo "$*" >> "${CURL_ARGS_FILE}"
output=""
while [[ $# -gt 0 ]]; do
    if [[ "$1" == "-o" ]]; then
        output="$2"
    fi
    shift
done
fake_dir="$(mktemp -d)"
cat > "${fake_dir}/crane" <<'CRANE'
#!/bin/bash
if [[ "$1" == "digest" ]]; then echo "${CRANE_STUB_DIGEST}"; else echo "fake crane ${CRANE_VERSION}"; fi
CRANE
chmod +x "${fake_dir}/crane"
tar -C "${fake_dir}" -czf "${output}" crane
rm -rf "${fake_dir}"
EOF
}

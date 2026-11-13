#!/usr/bin/env bash

# Test helpers specific to publish-distribution.bats. The distribution retag path resolves signing
# digests with crane (see publish-distribution.helpers.sh), so its tests install a fake crane. This
# lives here, local to the distribution tests, because the shared setup_command.bash is crane-free
# (the base publish path captures digests from buildx metadata instead). Requires _common_setup from
# setup_command to have run (it provides _create_stub and the PATH-first stub bin dir).

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

#!/bin/bash
source .buildkite/scripts/tooling.sh

set -euo pipefail

platform_type="$(uname)"
hw_type="$(uname -m)"
platform_type_lowercase="$(echo "${platform_type}" | tr '[:upper:]' '[:lower:]')"

check_platform_architecture() {
  case "${hw_type}" in
    "x86_64")
      arch_type="amd64"
      ;;
    "aarch64")
      arch_type="arm64"
      ;;
    "arm64")
      arch_type="arm64"
      ;;
    *)
    echo "The current platform/OS type is unsupported yet"
    ;;
  esac
}

create_bin_folder() {
    mkdir -p "${WORKSPACE}/bin"
}

add_bin_path(){
    create_bin_folder
    export PATH="${WORKSPACE}/bin:${PATH}"
}

with_go() {
    create_bin_folder
    check_platform_architecture

    echo "--- Install Golang"
    echo "GVM ${SETUP_GVM_VERSION} (platform ${platform_type_lowercase} arch ${arch_type}"
    retry 5 curl -sL -o "${WORKSPACE}/bin/gvm" "https://github.com/andrewkroh/gvm/releases/download/${SETUP_GVM_VERSION}/gvm-${platform_type_lowercase}-${arch_type}"

    chmod +x "${WORKSPACE}/bin/gvm"
    eval "$(gvm "$(cat .go-version)")"
    go version
    which go
    PATH="${PATH}:$(go env GOPATH)/bin"
    export PATH
}

with_mage() {
    check_platform_architecture

    if [[ "${platform_type_lowercase}" == "darwin" ]]; then
        # MacOS ARM VM images do not have golang installed by default
        with_go
    fi

    echo "--- Install mage"
    go install "github.com/magefile/mage@v${SETUP_MAGE_VERSION}"
    mage --version
}

with_go_junit_report() {
    echo "--- Install go-junit-report"
    go install github.com/jstemmer/go-junit-report/v2@latest
}

with_jq() {
    create_bin_folder
    check_platform_architecture
    # filename for versions <=1.6 is jq-linux64
    local binary="jq-${platform_type_lowercase}-${arch_type}"

    retry 5 curl -sL -o "${WORKSPACE}/bin/jq" "https://github.com/jqlang/jq/releases/download/jq-${JQ_VERSION}/${binary}"

    chmod +x "${WORKSPACE}/bin/jq"
    jq --version
}

# Required env variables:
#   WORKSPACE
#   SETUP_MAGE_VERSION
WORKSPACE=${WORKSPACE:-"$(pwd)"}
SETUP_MAGE_VERSION=${SETUP_MAGE_VERSION:-"1.14.0"}

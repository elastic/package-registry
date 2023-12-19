#!/bin/bash
source .buildkite/scripts/tooling.sh

set -euo pipefail

create_bin_folder() {
    mkdir -p "${WORKSPACE}/bin"
}

add_bin_path(){
    create_bin_folder
    export PATH="${WORKSPACE}/bin:${PATH}"
}

with_mage() {
    create_bin_folder
    retry 5 curl -sL -o "${WORKSPACE}/bin/mage.tar.gz" "https://github.com/magefile/mage/releases/download/v${SETUP_MAGE_VERSION}/mage_${SETUP_MAGE_VERSION}_Linux-64bit.tar.gz"

    tar -xvf "${WORKSPACE}/bin/mage.tar.gz" -C "${WORKSPACE}/bin"
    chmod +x "${WORKSPACE}/bin/mage"
    mage --version
}

with_go_junit_report() {
    go install github.com/jstemmer/go-junit-report/v2@latest
}

# Required env variables:
#   WORKSPACE
#   SETUP_MAGE_VERSION
WORKSPACE=${WORKSPACE:-"$(pwd)"}
SETUP_MAGE_VERSION=${SETUP_MAGE_VERSION:-"1.14.0"}

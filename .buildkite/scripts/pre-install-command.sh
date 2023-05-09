#!/bin/bash
set -euo pipefail

source .buildkite/scripts/tooling.sh

add_bin_path(){
    mkdir -p "${WORKSPACE}/bin"
    export PATH="${WORKSPACE}/bin:${PATH}"
}

with_mage() {
    mkdir -p "${WORKSPACE}/bin"
    retry 5 curl -sL -o "${WORKSPACE}/bin/mage.tar.gz" "https://github.com/magefile/mage/releases/download/v${SETUP_MAGE_VERSION}/mage_${SETUP_MAGE_VERSION}_Linux-64bit.tar.gz"

    tar -xvf "${WORKSPACE}/bin/mage.tar.gz" -C "${WORKSPACE}/bin"
    chmod +x "${WORKSPACE}/bin/mage"
    # mage --version
}

# Required env variables:
#   WORKSPACE
#   SETUP_MAGE_VERSION
WORKSPACE=${WORKSPACE:-"$(pwd)"}
SETUP_MAGE_VERSION=${SETUP_MAGE_VERSION:-"1.14.0"}

# Pre install:
uname -a
add_bin_path
with_mage

command=$1
"${command}"

#!/bin/bash

set -euo pipefail
SCRIPT_DIR="$( dirname "${BASH_SOURCE[0]}" &> /dev/null && pwd )"

usage() {
    echo "$0 [-b <bucket_name>] [-p <address>] [-h]"
    echo -e "\t-b <bucket_name>: Bucket name. Default: example"
    echo -e "\t-p <address>: Address of the package registry service. Default: localhost:8080"
    echo -e "\t-h: Show this message"
}

BUCKET_NAME="example"
ADDRESS="localhost:8080"

while getopts ":b:p:h" o; do
  case "${o}" in
    b)
      BUCKET_NAME="${OPTARG}"
      ;;
    p)
      ADDRESS="${OPTARG}"
      ;;
    h)
      usage
      exit 0
      ;;
    \?)
      echo "Invalid option ${OPTARG}"
      usage
      exit 1
      ;;
    :)
      echo "Missing argument for -${OPTARG}"
      usage
      exit 1
      ;;
  esac
done

cd "${SCRIPT_DIR}/.."
mage build

export STORAGE_EMULATOR_HOST="http://localhost:4443/"

export EPR_STORAGE_INDEXER_BUCKET_INTERNAL="gs://${BUCKET_NAME}"
export EPR_FEATURE_STORAGE_INDEXER="true"

export EPR_DISABLE_PACKAGE_VALIDATION="true"
export EPR_ADDRESS="${ADDRESS}"

./package-registry


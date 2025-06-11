#!/bin/bash
set -euo pipefail

cleanup() {
    echo "Cleaning up..."
    rm -f "${CURRENT_DIR}/package-registry"
}

trap cleanup EXIT
CURRENT_DIR="$(pwd)"
SCRIPT_DIR="$( cd -- "$(dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

usage() {
    echo "$0 [-b <bucket_name>] [-p <epr_address>] [-e <emulator_address>] [-i <index_path>] [-c <config_path>] [-h]"
    echo -e "\t-b <bucket_name>: Bucket name. Default: example"
    echo -e "\t-p <epr_address>: Address of the package registry service. Default: localhost:8080"
    echo -e "\t-e <emulator_address>: Address of the emulator host (fake GCS server). Default: localhost:4443"
    echo -e "\t-i <index_path>: Path to the search index JSON. Default: \"\""
    echo -e "\t-c <config_path>: Path to the configurastion file. Default: \"\""
    echo -e "\t-h: Show this message"
}

BUCKET_NAME="example"
ADDRESS="localhost:8080"
INDEX_PATH=""
EMULATOR_HOST="localhost:4443"
CONFIG_PATH="${SCRIPT_DIR}/../config.yml"

while getopts ":b:p:i:e:c:h" o; do
  case "${o}" in
    b)
      BUCKET_NAME="${OPTARG}"
      ;;
    p)
      ADDRESS="${OPTARG}"
      ;;
    i)
      INDEX_PATH="${OPTARG}"
      ;;
    e)
      EMULATOR_HOST="${OPTARG}"
      ;;
    c)
      CONFIG_PATH="${OPTARG}"
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

if [[ "$(pwd)" != "${CURRENT_DIR}" ]]; then
  mv ./package-registry "${CURRENT_DIR}"
  cd "${CURRENT_DIR}"
fi

export STORAGE_EMULATOR_HOST="${EMULATOR_HOST}"
export EPR_STORAGE_INDEXER_BUCKET_INTERNAL="gs://${BUCKET_NAME}"
if [[ "${INDEX_PATH}" != "" ]]; then
    export EPR_EMULATOR_INDEX_PATH="${INDEX_PATH}"
    export EPR_STORAGE_INDEXER_BUCKET_INTERNAL="gs://fake-package-storage-internal"
fi

export EPR_FEATURE_STORAGE_INDEXER="true"

export EPR_DISABLE_PACKAGE_VALIDATION="true"
export EPR_ADDRESS="${ADDRESS}"

# export EPR_LOG_LEVEL="debug"
export EPR_CONFIG="${CONFIG_PATH}"

./package-registry


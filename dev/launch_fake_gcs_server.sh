#!/bin/bash
set -euo pipefail

cleanup () {
    echo "Cleaning up..."
    docker-compose -f docker-compose-gcs.yml down -v
    echo "Removing source folder: ${SOURCE_FOLDER_PATH}"
    rm -rf "${SOURCE_FOLDER_PATH}"
    echo "Done."
    exit 0
}
trap cleanup EXIT

SCRIPT_DIR="$( cd -- "$(dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

DEFAULT_SOURCE_FOLDER_PATH="${SCRIPT_DIR}/../build/fakeserver/"
usage() {
    echo "$0 -i <index_path> [-b <bucket_name>] [-c <cursor>] [-p <bucket_path>] [-h]"
    echo -e "\t-i <index_path>: Path to the index file (JSON)."
    echo -e "\t-b <bucket_name>: Bucket name. Default: example"
    echo -e "\t-c <cursor>: Cursor id to create in the bucket. Default: \"1\""
    echo -e "\t-p <bucket_path>: Path to create the contents of the bucket. Default: \"${DEFAULT_SOURCE_FOLDER_PATH}"
    echo -e "\t-h: Show this message"
}

CURSOR="1"
BUCKET_NAME="example"
SOURCE_FOLDER_PATH="${DEFAULT_SOURCE_FOLDER_PATH}"
INDEX_PATH=""

while getopts ":b:c:i:p:h" o; do
  case "${o}" in
    b)
      BUCKET_NAME="${OPTARG}"
      ;;
    c)
      CURSOR="${OPTARG}"
      ;;
    i)
      INDEX_PATH="${OPTARG}"
      ;;
    p)
      SOURCE_FOLDER_PATH="${OPTARG}"
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

"${SCRIPT_DIR}/setup_bucket_gcs.sh" -b "${BUCKET_NAME}" -c "${CURSOR}" -i "${INDEX_PATH}" -p "${SOURCE_FOLDER_PATH}"

LOCAL_BUCKET_PATH="${SOURCE_FOLDER_PATH}"
if [[ ! "${SOURCE_FOLDER_PATH}" =~ ^/ ]]; then
    LOCAL_BUCKET_PATH="$(pwd)/${SOURCE_FOLDER_PATH}"
fi
export LOCAL_BUCKET_PATH

cd "${SCRIPT_DIR}"

# version fake-gcs-server
FAKE_GCS_SERVER_VERSION="$(grep fake-gcs-server ../go.mod | awk '{print $2}' | tr -d 'v')"
export FAKE_GCS_SERVER_VERSION


docker-compose -f docker-compose-gcs.yml up -d

docker-compose -f docker-compose-gcs.yml logs -f



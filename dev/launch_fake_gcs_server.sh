#!/bin/bash
set -euo pipefail

cleanup () {
    echo "Cleaning up..."
    docker-compose down -v
    rm -rf "${SOURCE_FOLDER_PATH}"
}
trap cleanup EXIT

SCRIPT_DIR="$( dirname "${BASH_SOURCE[0]}" &> /dev/null && pwd )"

usage() {
    echo "$0 -i <index_path> [-b <bucket_name>] [-c <cursor>] [-h]"
    echo -e "\t-i <index_path>: Path to the index file (JSON)."
    echo -e "\t-b <bucket_name>: Bucket name. Default: example"
    echo -e "\t-c <cursor>: Cursor id to create in the bucket. Default: \"1\""
    echo -e "\t-h: Show this message"
}

CURSOR="1"
BUCKET_NAME="example"
SOURCE_FOLDER_PATH="${SCRIPT_DIR}/../build/fakeserver/"
INDEX_PATH=""

while getopts ":b:c:i:h" o; do
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

if [[ "${INDEX_PATH}" == "" ]]; then
    echo "Missing index path parameter"
    usage
    exit 1
fi

METADATA_FOLDER="${SOURCE_FOLDER_PATH}/${BUCKET_NAME}/v2/metadata"
echo "Cleaning up metadata folder: ${METADATA_FOLDER}"
rm -rf "${METADATA_FOLDER}"

CURSOR_FOLDER="${METADATA_FOLDER}/${CURSOR}"
mkdir -p "${CURSOR_FOLDER}"

cp "${INDEX_PATH}" "${CURSOR_FOLDER}/search-index-all.json"

cat <<EOF > "${METADATA_FOLDER}/cursor.json"
{
  "current": "${CURSOR}"
}
EOF

echo "Contents of bucket ${BUCKET_NAME}"
tree "${SOURCE_FOLDER_PATH}"

cd "${SCRIPT_DIR}"

docker-compose up -d

docker-compose logs -f



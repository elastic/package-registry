#!/bin/bash
set -euo pipefail

usage() {
    echo "$0 -c <cursor> -i <index_path> [-b <bucket_name>] [-e <emulator_address>] [-h]"
    echo -e "\t-c <cursor>: Cursor to set the new index."
    echo -e "\t-i <index_path>: Path to the search index JSON."
    echo -e "\t-b <bucket_name>: Bucket name. Default: example"
    echo -e "\t-e <emulator_address>: Address of the emulator host (fake GCS server). Default: http://localhost:4443"
    echo -e "\t-h: Show this message"
}

BUCKET_NAME="example"
INDEX_PATH=""
EMULATOR_HOST="http://localhost:4443"
CURSOR=""

while getopts ":b:i:e:c:h" o; do
  case "${o}" in
    b)
      BUCKET_NAME="${OPTARG}"
      ;;
    i)
      INDEX_PATH="${OPTARG}"
      ;;
    e)
      EMULATOR_HOST="${OPTARG}"
      ;;
    c)
      CURSOR="${OPTARG}"
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

if [[ "${CURSOR}" == "" ]]; then
    echo "Missing cursor"
    usage
    exit 1
fi

BASE_URL="${EMULATOR_HOST}/upload/storage/v1/b/${BUCKET_NAME}/o"
curl -X POST \
    --data-binary "@${INDEX_PATH}" \
    -H "Content-Type: application/json" \
    "${BASE_URL}?uploadType=media&name=v2%2Fmetadata%2F${CURSOR}%2Fsearch-index-all.json"

cat << EOF > cursor-new.json
{
  "current": "${CURSOR}"
}
EOF

echo "Setting cursor as:"
cat cursor-new.json

curl -X POST \
    --data-binary @cursor-new.json \
    -H "Content-Type: application/json" \
    "${BASE_URL}?uploadType=media&name=v2%2Fmetadata%2Fcursor.json"

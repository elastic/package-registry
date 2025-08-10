#!/usr/bin/env bash

set -euo pipefail

usage() {
    echo "$0[-i <iterations>] -f <script_file> [-h]"
    echo -e "\t-i <iterations>: Number of iterations to run load test. Default: \"10\""
    echo -e "\t-f <script_file>: Path to script with the load test definition."
    echo -e "\t-h: Show this message"
}


if ! command -v k6 > /dev/null ; then
    echo "Missing k6 binary "
    echo "- Follow instructions on https://grafana.com/docs/k6/latest/set-up/install-k6/ to install k6"
    exit 1
fi

ITERATIONS=10
SCRIPT_FILE=""

while getopts ":i:f:h" o; do
  case "${o}" in
    i)
      ITERATIONS="${OPTARG}"
      ;;
    f)
      SCRIPT_FILE="${OPTARG}"
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

if [[ "${SCRIPT_FILE}" == "" ]] ; then
    echo "Missing script file"
    exit 1
fi

k6 run \
    --iterations "${ITERATIONS}"  \
    --summary-mode full \
    "${SCRIPT_FILE}"

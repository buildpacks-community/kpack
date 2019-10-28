#!/usr/bin/env bash

cd $(dirname "${BASH_SOURCE[0]}")/..

if [ -z "$1" ]; then
  echo "Usage: ./hack/apply.sh <DOCKER_REPO>"
  exit 0
fi

source hack/common.sh

set -e

TMP_DIR="$(mktemp -d)"

compile $1 ${TMP_DIR}/out.yaml

kubectl apply -f ${TMP_DIR}/out.yaml
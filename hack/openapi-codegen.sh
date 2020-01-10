#!/usr/bin/env bash

set -e
set -o errexit
set -o nounset
set -o pipefail

OPENAPI_GEN_BIN=${1:-openapi-gen}

SCRIPT_ROOT=$(realpath $(dirname ${BASH_SOURCE})/..)

TMP_DIR="$(mktemp -d)"
trap 'rm -rf ${TMP_DIR}' EXIT
export GOPATH=${GOPATH:-${TMP_DIR}}

TMP_REPO_PATH="${TMP_DIR}/src/github.com/pivotal/kpack"
mkdir -p "$(dirname "${TMP_REPO_PATH}")" && ln -s "${SCRIPT_ROOT}" "${TMP_REPO_PATH}"

${OPENAPI_GEN_BIN} \
  -h "${SCRIPT_ROOT}"/hack/boilerplate/boilerplate.go.txt \
  -i github.com/pivotal/kpack/pkg/apis/build/v1alpha1,github.com/pivotal/kpack/pkg/apis/experimental/v1alpha1,github.com/pivotal/kpack/pkg/apis/core/v1alpha1 \
  -p github.com/pivotal/kpack/pkg/openapi

go run ${SCRIPT_ROOT}/hack/openapi/main.go 1> ${SCRIPT_ROOT}/api/openapi-spec/swagger.json
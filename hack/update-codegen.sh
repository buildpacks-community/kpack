#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(realpath $(dirname ${BASH_SOURCE})/..)

pushd $SCRIPT_ROOT
  go mod vendor
popd
trap 'rm -rf $SCRIPT_ROOT/vendor' EXIT

CODEGEN_PKG=${CODEGEN_PKG:-$(cd "${SCRIPT_ROOT}"; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

TMP_DIR="$(mktemp -d)"
trap 'rm -rf ${TMP_DIR} && rm -rf $SCRIPT_ROOT/vendor' EXIT
export GOPATH=${GOPATH:-${TMP_DIR}}

TMP_REPO_PATH="${TMP_DIR}/src/github.com/pivotal/kpack"
mkdir -p "$(dirname "${TMP_REPO_PATH}")" && ln -s "${SCRIPT_ROOT}" "${TMP_REPO_PATH}"

bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy,client,informer,lister" \
  github.com/pivotal/kpack/pkg/client github.com/pivotal/kpack/pkg/apis \
  "build:v1alpha1,v1alpha2" \
  --output-base "${TMP_DIR}/src" \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate/boilerplate.go.txt

bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy" \
  github.com/pivotal/kpack/pkg/client github.com/pivotal/kpack/pkg/apis \
  "core:v1alpha1" \
  --output-base "${TMP_DIR}/src" \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate/boilerplate.go.txt

bash "${CODEGEN_PKG}"/generate-groups.sh "deepcopy" \
  github.com/pivotal/kpack/pkg/client github.com/pivotal/kpack/pkg/apis \
  "fake" \
  --output-base "${TMP_DIR}/src" \
  --go-header-file "${SCRIPT_ROOT}"/hack/boilerplate/boilerplate.go.txt

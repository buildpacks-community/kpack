#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(realpath $(dirname ${BASH_SOURCE})/..)

CODEGEN_PKG=$(go list -m -mod=readonly -f "{{.Dir}}" k8s.io/code-generator)
source "${CODEGEN_PKG}"/kube_codegen.sh

kube::codegen::gen_helpers \
  --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate/boilerplate.go.txt \
  "${SCRIPT_ROOT}/pkg/apis"

kube::codegen::gen_client \
  --boilerplate "${SCRIPT_ROOT}"/hack/boilerplate/boilerplate.go.txt \
  --output-dir "${SCRIPT_ROOT}/pkg/client" \
  --output-pkg "github.com/pivotal/kpack/pkg/client" \
  --with-watch \
  "${SCRIPT_ROOT}/pkg/apis"

go mod tidy

#!/usr/bin/env bash

set -e
set -o errexit
set -o nounset
set -o pipefail

OPENAPI_GEN_BIN=${1:-openapi-gen}

ROOT=$(realpath $(dirname ${BASH_SOURCE})/..)

cd "$ROOT"

${OPENAPI_GEN_BIN} \
  -h ./hack/boilerplate/boilerplate.go.txt \
  -i github.com/pivotal/kpack/pkg/apis/build/v1alpha1,github.com/pivotal/kpack/pkg/apis/core/v1alpha1 \
  -p ./pkg/openapi \
  -o ./

go run ./hack/openapi/main.go 1> ./api/openapi-spec/swagger.json

cd -

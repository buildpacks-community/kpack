#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

ROOT=$(realpath $(dirname ${BASH_SOURCE})/..)

cd "$ROOT"

go run k8s.io/kube-openapi/cmd/openapi-gen \
  --go-header-file ./hack/boilerplate/boilerplate.go.txt \
  --output-pkg openapi \
  --output-dir ./pkg/openapi \
    github.com/pivotal/kpack/pkg/apis/build/v1alpha1 github.com/pivotal/kpack/pkg/apis/build/v1alpha2 github.com/pivotal/kpack/pkg/apis/core/v1alpha1 

# VolatileTime has custom json encoding/decoding that does not map to a proper json schema. Use a basic string instead.
sed -i.old 's/Ref\:         ref(\"github.com\/pivotal\/kpack\/pkg\/apis\/core\/v1alpha1.VolatileTime\"),/Type: []string{\"string\"}, Format: \"\",/g' pkg/openapi/generated.openapi.go
sed -i.old 's/Ref\:         ref(\"github.com\/pivotal\/kpack\/pkg\/apis\/core\/v1alpha2.VolatileTime\"),/Type: []string{\"string\"}, Format: \"\",/g' pkg/openapi/generated.openapi.go

go run ./hack/openapi/main.go 1> ./api/openapi-spec/swagger.json
rm -f pkg/openapi/generated.openapi.go.old

cd -

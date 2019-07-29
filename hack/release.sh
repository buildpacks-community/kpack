#!/usr/bin/env bash

cd $(dirname "${BASH_SOURCE[0]}")/..

set -o errexit
set -o nounset
set -o pipefail

if [ -z "$2" ]; then
  echo "Usage: ./hack/release.sh <registry> <release.yml>"
  exit 0
fi

source hack/common.sh

registry=$1
release_yaml=$2

controller_image=${registry}/controller
build_init_image=${registry}/build-init

pack_build ${controller_image} "./cmd/controller"
controller_image=${resolved_image_name}

pack_build ${build_init_image} "./cmd/build-init"
build_init_image=${resolved_image_name}

git_init_image=gcr.io/cf-build-service-dev-219913/git-init@sha256:a0b587d97503ccce2109dcaa1462ff62be040388baeb3425507b300b9ecb3b86
cred_init_image=gcr.io/pivotal-knative/github.com/knative/build/cmd/creds-init@sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895
nop_image=gcr.io/pivotal-knative/github.com/knative/build/cmd/nop@sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4

ytt -f config/. -v controller_image=${controller_image} -v build_init_image=${build_init_image} -v git_init_image=${git_init_image} -v cred_init_image=${cred_init_image} -v nop_image=${nop_image} > ${release_yaml}
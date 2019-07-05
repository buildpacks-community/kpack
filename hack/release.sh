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

ytt -f config/. -v controller_image=${controller_image} -v build_init_image=${build_init_image} > ${release_yaml}
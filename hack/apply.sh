#!/usr/bin/env bash

cd $(dirname "${BASH_SOURCE[0]}")/..

if [ -z "$1" ]; then
  echo "Usage: ./hack/apply.sh <DOCKER_REPO>"
  exit 0
fi

source hack/common.sh

docker_repo=$1
controller_image=${docker_repo}/controller
build_init_image=${docker_repo}/build-init

pack_build ${controller_image} "./cmd/controller"
controller_image=${resolved_image_name}

pack_build ${build_init_image} "./cmd/build-init"
build_init_image=${resolved_image_name}

ytt -f config/. -v controller_image=${controller_image} -v build_init_image=${build_init_image} | kubectl apply -f -
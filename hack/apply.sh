#!/usr/bin/env bash

cd $(dirname "${BASH_SOURCE[0]}")/..

if [ -z "$1" ]; then
  echo "Usage: ./hack/apply.sh <DOCKER_REPO>"
  exit 0
fi

source hack/common.sh

set -e

docker_repo=$1
controller_image=${docker_repo}/controller
build_init_image=${docker_repo}/build-init

pack_build ${controller_image} "./cmd/controller"
controller_image=${resolved_image_name}

pack_build ${build_init_image} "./cmd/build-init"
build_init_image=${resolved_image_name}

nop_image=gcr.io/pivotal-knative/github.com/knative/build/cmd/nop@sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4

ytt -f config/. -v controller_image=${controller_image} -v build_init_image=${build_init_image} -v nop_image=${nop_image} | kubectl apply -f -
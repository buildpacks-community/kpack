#!/usr/bin/env bash

cd $(dirname "${BASH_SOURCE[0]}")/..

if [ -z "$1" ]; then
  echo "Usage: ./hack/apply.sh <DOCKER_REPO>"
  exit 0
fi

source hack/common.sh

set -e

docker_repo=$1
controller_image=${docker_repo}/controller@sha256:14db767ec0d8da83a82f31944bf352056aaaa72d2b6809f6dc5e26eef49b5bfe
build_init_image=${docker_repo}/build-init@sha256:5f5c92cb3427d692dfcd5320ae903e3458194c04010b9a0041f80d675e05ef08
build_webhook=${docker_repo}/build-webhook

keydir="$(mktemp -d)"
function clean_dir(){
  rm -rf "${keydir}"
}
trap clean_dir EXIT
hack/generate-keys.sh "${keydir}"

pack_build "${controller_image}" "./cmd/controller"
controller_image=${resolved_image_name}

pack_build "${build_init_image}" "./cmd/build-init"
build_init_image=${resolved_image_name}

pack_build "${build_webhook}" "./cmd/build-defaults-webhook"
build_webhook=${resolved_image_name}

nop_image=gcr.io/pivotal-knative/github.com/knative/build/cmd/nop@sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4

ytt -f config/. \
  -f "${keydir}/webhook-server-tls.crt" \
  -f "${keydir}/webhook-server-tls.key" \
  -f "${keydir}/ca.crt" \
  -v pod_webhook_image="${build_webhook}" \
  -v controller_image="${controller_image}" \
  -v build_init_image="${build_init_image}" \
  -v nop_image=${nop_image} | kubectl apply -f -
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

# e.g. gcr.io/cf-build-service-public/kpack
registry=$1
# e.g. path/to/release.yml
release_yaml=$2
mkdir -p $(dirname ${release_yaml})

# Overrides registry=$1 for Docker Hub images
# e.g. IMAGE_PREFIX=username/kpack-
IMAGE_PREFIX=${IMAGE_PREFIX:-"${registry}/"}

controller_image=${IMAGE_PREFIX}controller
build_init_image=${IMAGE_PREFIX}build-init
source_init_image=${IMAGE_PREFIX}source-init

pack_build ${controller_image} "./cmd/controller"
controller_image=${resolved_image_name}

pack_build ${build_init_image} "./cmd/build-init"
build_init_image=${resolved_image_name}

pack_build ${source_init_image} "./cmd/source-init"
source_init_image=${resolved_image_name}

cred_init_image=gcr.io/pivotal-knative/github.com/knative/build/cmd/creds-init@sha256:2bc85afc0ee0aec012b3889cf5f2e9690bb504c9d19ce90add2f415b85990895
nop_image=gcr.io/pivotal-knative/github.com/knative/build/cmd/nop@sha256:dc7e5e790001c71c2cfb175854dd36e65e0b71c58294b331a519be95bdec4ef4

ytt -f config/. -v controller_image=${controller_image} -v build_init_image=${build_init_image} -v source_init_image=${source_init_image} -v cred_init_image=${cred_init_image} -v nop_image=${nop_image} > ${release_yaml}
#!/bin/bash

function pack_build() {
    image=$1
    target=$2
    pack_args=${@:3}
    builder="gcr.io/cf-build-service-public/ci/kpack-builder" # builder used ci

    pack build ${image} --builder ${builder} -e BP_GO_TARGETS=${target} ${pack_args} --publish --trust-builder

    docker pull ${image}
    resolved_image_name=$(docker inspect ${image} --format '{{index .RepoDigests 0}}' )
}

function lifecycle_image_build() {
    image=$1
    go run hack/lifecycle/main.go --tag=${image}

    docker pull ${image}
    resolved_image_name=$(docker inspect ${image} --format '{{index .RepoDigests 0}}' )
}

function compile() {
  registry=$1
  output=$2
  # Overrides registry=$1 for Docker Hub images
  # e.g. IMAGE_PREFIX=username/kpack-
  IMAGE_PREFIX=${IMAGE_PREFIX:-"${registry}/"}

  controller_image=${IMAGE_PREFIX}controller
  webhook_image=${IMAGE_PREFIX}webhook
  build_init_image=${IMAGE_PREFIX}build-init
  rebase_image=${IMAGE_PREFIX}rebase
  completion_image=${IMAGE_PREFIX}completion
  lifecycle_image=${IMAGE_PREFIX}lifecycle

  pack_build ${controller_image} "./cmd/controller" -e BP_BUILD_LIBGIT2=true -e BP_GO_BUILD_FLAGS='-tags="static"'

  controller_image=${resolved_image_name}

  pack_build ${webhook_image} "./cmd/webhook"
  webhook_image=${resolved_image_name}

  pack_build ${build_init_image} "./cmd/build-init" -e BP_BUILD_LIBGIT2=true -e BP_GO_BUILD_FLAGS='-tags="static"'
  build_init_image=${resolved_image_name}

  pack_build ${rebase_image} "./cmd/rebase"
  rebase_image=${resolved_image_name}

  pack_build ${completion_image} "./cmd/completion"
  completion_image=${resolved_image_name}

  lifecycle_image_build ${lifecycle_image}
  lifecycle_image=${resolved_image_name}

  ytt -f config/. \
    -v controller_image=${controller_image} \
    -v webhook_image=${webhook_image} \
    -v build_init_image=${build_init_image} \
    -v rebase_image=${rebase_image} \
    -v completion_image=${completion_image} \
    -v lifecycle_image=${lifecycle_image} > $output
}

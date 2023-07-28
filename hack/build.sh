#!/bin/bash

function lifecycle_image_build() {
    image=$1
    go run hack/lifecycle/main.go --tag=${image}
}

function generate_kbld_config_pack() {
  path=$1
  registry=$2

  controller_args=("--env" "BP_GO_TARGETS=./cmd/controller")
  controller_args+=($buildArgs)
  controller_args="${controller_args[@]}";

  webhook_args=("--env" "BP_GO_TARGETS=./cmd/webhook")
  webhook_args+=($buildArgs)
  webhook_args="${webhook_args[@]}";

  build_init_args=("--env" "BP_GO_TARGETS=./cmd/build-init")
  build_init_args+=($buildArgs)
  build_init_args="${build_init_args[@]}";

  build_waiter_args=("--env" "BP_GO_TARGETS=./cmd/build-waiter")
  build_waiter_args+=($buildArgs)
  build_waiter_args="${build_waiter_args[@]}";

  rebase_args=("--env" "BP_GO_TARGETS=./cmd/rebase")
  rebase_args+=($buildArgs)
  rebase_args="${rebase_args[@]}";

  completion_args=("--env" "BP_GO_TARGETS=./cmd/completion")
  completion_args+=($buildArgs)
  completion_args="${completion_args[@]}";

  cat <<EOT > $path
  apiVersion: kbld.k14s.io/v1alpha1
  kind: Config
  sources:
  - image: controller
    path: .
    pack:
      build:
        builder: paketobuildpacks/builder-jammy-tiny
        rawOptions: [${controller_args// /,}]
  - image: webhook
    path: .
    pack:
      build:
        builder: paketobuildpacks/builder-jammy-tiny
        rawOptions: [${webhook_args// /,}]
  - image: build-init
    path: .
    pack:
      build:
        builder: paketobuildpacks/builder-jammy-tiny
        rawOptions: [${build_init_args// /,}]
  - image: build-waiter
    path: .
    pack:
      build:
        builder: paketobuildpacks/builder-jammy-tiny
        rawOptions: [${build_waiter_args// /,}]
  - image: rebase
    path: .
    pack:
      build:
        builder: paketobuildpacks/builder-jammy-tiny
        rawOptions: [${rebase_args// /,}]
  - image: completion
    path: .
    pack:
      build:
        builder: paketobuildpacks/builder-jammy-tiny
        rawOptions: [${completion_args// /,}]
  overrides:
  - image: build-init-windows
    newImage: build-init-windows
    preresolved: true
  - image: completion-windows
    newImage: completion-windows
    preresolved: true
  - image: lifecycle
    newImage: $lifecycle_image
  destinations:
  - image: controller
    newImage: $controller_image
  - image: webhook
    newImage: $webhook_image
  - image: build-init
    newImage: $build_init_image
  - image: build-waiter
    newImage: $build_waiter_image
  - image: rebase
    newImage: $rebase_image
  - image: completion
    newImage: $completion_image
EOT
}

function generate_kbld_config_ko() {
  path=$1
  registry=$2

  args=("--disable-optimizations")
  args+=($buildArgs)
  args="${args[@]}";

  cat <<EOT > $path
  apiVersion: kbld.k14s.io/v1alpha1
  kind: Config
  sources:
  - image: controller
    path: cmd/controller
    ko:
      build:
        rawOptions: [${args// /,}]
  - image: webhook
    path: cmd/webhook
    ko:
      build:
        rawOptions: [${args// /,}]
  - image: build-init
    path: cmd/build-init
    ko:
      build:
        rawOptions: [${args// /,}]
  - image: build-waiter
    path: cmd/build-waiter
    ko:
      build:
        rawOptions: [${args// /,}]
  - image: rebase
    path: cmd/rebase
    ko:
      build:
        rawOptions: [${args// /,}]
  - image: completion
    path: cmd/completion
    ko:
      build:
        rawOptions: [${args// /,}]
  overrides:
  - image: build-init-windows
    newImage: build-init-windows
    preresolved: true
  - image: completion-windows
    newImage: completion-windows
    preresolved: true
  - image: lifecycle
    newImage: $lifecycle_image
  destinations:
  - image: controller
    newImage: $controller_image
  - image: webhook
    newImage: $webhook_image
  - image: build-init
    newImage: $build_init_image
  - image: build-waiter
    newImage: $build_waiter_image
  - image: rebase
    newImage: $rebase_image
  - image: completion
    newImage: $completion_image


EOT
}

function compile() {
  type=$1
  registry=$2
  output=$3

  # Overrides registry=$1 for Docker Hub images
  # e.g. IMAGE_PREFIX=username/kpack-
  IMAGE_PREFIX=${IMAGE_PREFIX:-"${registry}/"}
  controller_image=${IMAGE_PREFIX}controller
  webhook_image=${IMAGE_PREFIX}webhook
  build_init_image=${IMAGE_PREFIX}build-init
  build_waiter_image=${IMAGE_PREFIX}build-waiter
  rebase_image=${IMAGE_PREFIX}rebase
  completion_image=${IMAGE_PREFIX}completion
  lifecycle_image=${IMAGE_PREFIX}lifecycle

  echo "Building Lifecycle"
  lifecycle_image_build ${lifecycle_image}

  echo "Generating kbld config"
  temp_dir=$(mktemp -d)
  kbld_config_path="${temp_dir}/kbld-config"
  if [ $type = "ko" ]; then
    generate_kbld_config_ko $kbld_config_path $registry
  elif [ $type = "pack" ]; then
    generate_kbld_config_pack $kbld_config_path $registry
  else
    echo "invalid build type, either 'pack' or 'ko' is allowed"
    exit 1
  fi

  echo "Building Images"
  ytt -f config | kbld -f $kbld_config_path -f- > $output
}
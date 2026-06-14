#!/bin/bash

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
    - image: lifecycle
      newImage: mirror.gcr.io/buildpacksio/lifecycle
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

function generate_ko_config() {
  ko_config_path=$1

  prefix="github.com/pivotal/kpack/pkg/apis/build/v1alpha2"
  cat <<EOT > $ko_config_path
  defaultBaseImage: paketobuildpacks/run-jammy-tiny

  builds:
  - id: controller
    ldflags:
    - -X ${prefix}.CompletionCommand=/ko-app/completion
    - -X ${prefix}.PrepareCommand=/ko-app/build-init
    - -X ${prefix}.RebaseCommand=/ko-app/rebase
EOT
}

function generate_kbld_config_ko() {
  kbld_config_path=$1
  ko_config_path=$2
  registry=$3

  generate_ko_config $ko_config_path

  args=("--disable-optimizations")
  args+=($buildArgs)
  args="${args[@]}";

  cat <<EOT > $kbld_config_path
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
  - image: lifecycle
    newImage: mirror.gcr.io/buildpacksio/lifecycle
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

# generate_kbld_config_ko_multiarch builds each binary into a genuine multi-arch
# image index by invoking ko directly, then records the resulting index digests
# as kbld overrides.
#
# kbld's ko integration builds for the host platform only (it collapses ko's
# multi-platform build down to a single, host-arch manifest), so the published
# images don't run on other architectures (e.g. arm64 clusters fail with
# "exec format error"). Calling `ko build --platform=...` ourselves produces a
# real OCI image index covering every requested platform; feeding those refs to
# kbld as overrides keeps the rest of the release pipeline (manifest rewriting,
# digest pinning) unchanged.
function generate_kbld_config_ko_multiarch() {
  kbld_config_path=$1
  ko_config_path=$2
  registry=$3
  platforms=$4

  generate_ko_config $ko_config_path

  # Each entry is "kbld-image-name:cmd-dir:destination-image-ref".
  images=(
    "controller:cmd/controller:$controller_image"
    "webhook:cmd/webhook:$webhook_image"
    "build-init:cmd/build-init:$build_init_image"
    "build-waiter:cmd/build-waiter:$build_waiter_image"
    "rebase:cmd/rebase:$rebase_image"
    "completion:cmd/completion:$completion_image"
  )

  cat <<EOT > $kbld_config_path
  apiVersion: kbld.k14s.io/v1alpha1
  kind: Config
  overrides:
  - image: lifecycle
    newImage: mirror.gcr.io/buildpacksio/lifecycle
EOT

  for entry in "${images[@]}"; do
    name=${entry%%:*}
    rest=${entry#*:}
    dir=${rest%%:*}
    dest=${rest#*:}

    echo "Building multi-arch image ($platforms) for $name via ko" >&2
    # ko pushes the built index to KO_DOCKER_REPO (with --bare, the repo is
    # used verbatim) and prints the resulting digest-pinned index ref.
    ref=$(KO_DOCKER_REPO="$dest" KO_CONFIG_PATH="$ko_config_path" \
      ko build --bare --platform="$platforms" --disable-optimizations "./$dir")

    cat <<EOT >> $kbld_config_path
  - image: $name
    newImage: $ref
EOT
  done
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

  echo "Generating kbld config"
  temp_dir=$(mktemp -d)
  kbld_config_path="${temp_dir}/kbld-config"
  ko_config_path="${temp_dir}/.ko.yaml"
  # PLATFORMS optionally requests a multi-arch (e.g. "linux/amd64,linux/arm64")
  # build. When unset, the native, single-arch behavior is preserved.
  PLATFORMS=${PLATFORMS:-}
  if [ $type = "ko" ]; then
    if [ -n "$PLATFORMS" ]; then
      generate_kbld_config_ko_multiarch $kbld_config_path $ko_config_path $registry $PLATFORMS
    else
      generate_kbld_config_ko $kbld_config_path $ko_config_path $registry
    fi
  elif [ $type = "pack" ]; then
    generate_kbld_config_pack $kbld_config_path $registry
  else
    echo "invalid build type, either 'pack' or 'ko' is allowed"
    exit 1
  fi

  echo "Building Images"
 ytt -f config | KO_CONFIG_PATH="$ko_config_path" kbld -f $kbld_config_path -f- > $output
}
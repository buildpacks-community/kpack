#!/usr/bin/env bash

set -eu
set -o pipefail

cd $(dirname "${BASH_SOURCE[0]}")/..

function usage() {
  cat <<-USAGE
local.sh [OPTIONS]

Builds and generates a deployment yaml for kpack. This only builds linux images.

Prerequisites:
- pack or ko installed
- kubectl installed
- docker login to your registry

OPTIONS
  --help                          -h  prints the command usage
  --build-type <buildType>            build system to use. valid options are pack or ko.
  --registry <registry>               registry to publish built images to (e.g. gcr.io/myproject/my-repo or my-dockerhub-username)
  --output <output>                   filepath for generated deployment yaml. defaults to a temp file
  --apply                             (boolean) apply deployment yaml to current kubectl context
  --build-args                        argument to pass to build system (i.e --clear-cache for pack)

USAGE
}

function main() {
  local buildType registry output apply buildArgs
  apply="false"

  while [[ "${#}" != 0 ]]; do
    case "${1}" in
      --help|-h)
        shift 1
        usage
        exit 0
        ;;

      --build-type)
        buildType=("${2}")
        shift 2
        ;;

      --registry)
        registry=("${2}")
        shift 2
        ;;

      --output)
        output=("${2}")
        shift 2
        ;;

      --build-args)
      buildArgs=("${2}")
      shift 2
      ;;

      --apply)
        apply="true"
        shift 1
        ;;

      "")
        # skip if the argument is empty
        shift 1
        ;;

      *)
        echo -e "unknown argument \"${1}\"" >&2
        exit 1
    esac
  done

 if [ -z "${registry:-}" ]; then
  echo "--registry is required"
  usage
  exit 1
 fi

 if [ -z "${buildType:-}" ]; then
   echo "--buildType is required"
   usage
   exit 1
 fi

 if [ -z "${output:-}" ]; then
   tmp_dir="$(mktemp -d)"
   output=${tmp_dir}/out.yaml
   echo "will write to $output"
 fi

 if [ -z "${buildArgs:-}" ]; then
    buildArgs=""
 fi

 source hack/build.sh
 compile $buildType $registry $output

 if "${apply}"; then
  echo "Applying $output to cluster"
  kubectl apply -f $output
 fi

}

main "${@:-}"
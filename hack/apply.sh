#!/usr/bin/env bash

if [ -z "$1" ]; then
  echo "Usage: ./hack/apply.sh <controller/image>"
  exit 0
fi

CONTROLLER_IMAGE=$1

cd $(dirname "${BASH_SOURCE[0]}")/..

pack build ${CONTROLLER_IMAGE} --builder heroku/buildpacks --publish

ytt -f config/.  -v controller_image=${CONTROLLER_IMAGE} | kubectl apply -f -
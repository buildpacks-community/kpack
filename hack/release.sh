#!/usr/bin/env bash

if [ -z "$2" ]; then
  echo "Usage: ./hack/release.sh <controller/image> <release.yml>"
  exit 0
fi

controller_image=$1
release_yml=$2

cd $(dirname "${BASH_SOURCE[0]}")/..

pack build ${controller_image} --builder heroku/buildpacks --publish | tee pack-output

resolved_image_name=$(cat pack-output | grep "\*\*\* Image" | cut -d " " -f 4)
rm pack-output

ytt -f config/.  -v controller_image="${resolved_image_name}" > ${release_yml}
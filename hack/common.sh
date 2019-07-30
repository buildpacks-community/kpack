#!/bin/bash

function pack_build() {
    image=$1
    target=$2
    builder="cloudfoundry/cnb:bionic"

    pack build ${image} --builder ${builder} -e BP_GO_TARGETS=${target} --publish --clear-cache | tee pack-output

    resolved_image_name=$(cat pack-output | grep "\*\*\* Image" | cut -d " " -f 4)
    rm pack-output
}

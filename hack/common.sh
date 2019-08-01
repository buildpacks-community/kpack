#!/bin/bash

function pack_build() {
    image=$1
    target=$2
    builder="cloudfoundry/cnb:bionic"
    run_image="cloudfoundry/build:base-cnb"

    pack build ${image} --builder ${builder} --run-image ${run_image} -e BP_GO_TARGETS=${target} --publish --clear-cache | tee pack-output

    resolved_image_name=$(cat pack-output | grep -A1 "\*\*\* Images:" | grep -v "\*\*\* Images:" | awk '{print $2}')
    resolved_image_digest=$(cat pack-output | grep "\*\*\* Digest:" | awk '{print $4}')
    rm pack-output
    resolved_image_name="$resolved_image_name@$resolved_image_digest"
}

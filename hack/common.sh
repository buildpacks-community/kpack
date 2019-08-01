#!/bin/bash

function pack_build() {
    image=$1
    target=$2
    builder="cloudfoundry/cnb:bionic"

    pack build ${image} --builder ${builder} -e BP_GO_TARGETS=${target} --publish --clear-cache | tee pack-output

    docker pull ${image}
    resolved_image_name=$(docker inspect ${image} --format "$image@{{.Config.Image}}" )
    rm pack-output
}

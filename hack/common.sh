#!/bin/bash

function pack_build() {
    image=$1
    target=$2
    builder="cloudfoundry/cnb:bionic"
    run_image="cloudfoundry/build:base-cnb"

    pack build ${image} --builder ${builder} --run-image ${run_image} -e BP_GO_TARGETS=${target} --publish

    docker pull ${image}
    resolved_image_name=$(docker inspect ${image} --format '{{index .RepoDigests 0}}' )
}

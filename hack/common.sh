#!/bin/bash

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

  lifecycle_image=${IMAGE_PREFIX}lifecycle

  lifecycle_image_build ${lifecycle_image}
  lifecycle_image=${resolved_image_name}
  
  (echo "#@data/values"; kbld --images-annotation=false -f ./hack/images.yaml) > /tmp/kbld-images.yaml
    
  ytt -f config/. -f ./hack/values.yaml -f /tmp/kbld-images.yaml -v lifecycle.image="${lifecycle_image}"  > "${output}"
}
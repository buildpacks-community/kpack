#!/bin/bash

export KPACK_lifecycle__image="gcr.io/cf-build-service-public/kpack/lifecycle"

ytt -f config/ --data-values-env-yaml KPACK

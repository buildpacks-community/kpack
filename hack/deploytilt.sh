#!/bin/bash

sed -i -e 's/^/#/' config/config-logging.yaml

export KPACK_lifecycle__image="gcr.io/cf-build-service-public/kpack/lifecycle"

ytt -f config/ --data-values-env-yaml KPACK

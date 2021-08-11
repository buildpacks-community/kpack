# Go parameters
GOCMD?=go

.PHONY: default
default: unit

.PHONY: unit
unit:
	$(GOCMD) test -v -count=1 -parallel=1 -timeout=0 ./pkg/...

.PHONY: unit-ci
unit-ci:
	$(GOCMD) test -v -count=1 -parallel=1 -timeout=0 ./pkg/... -coverprofile=coverage.txt -covermode=atomic

.PHONY: e2e
e2e:
	$(GOCMD) test -v -count=1 -timeout=0 ./test/...

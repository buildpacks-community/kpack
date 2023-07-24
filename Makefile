# Go parameters
GOCMD?=go

all: unit

unit:
	$(GOCMD) test -v -count=1 -parallel=1 -timeout=0 ./pkg/...

unit-ci:
	$(GOCMD) test -v -count=1 -parallel=1 -timeout=0 ./pkg/... -coverprofile=coverage.txt -covermode=atomic

e2e:
	$(GOCMD) test --timeout=30m -v ./test/...

.PHONY: unit

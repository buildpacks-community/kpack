# Go parameters
GOCMD?=go

all: unit

unit:
	$(GOCMD) test ./pkg/...

unit-ci:
	$(GOCMD) test ./pkg/... -coverprofile=coverage.txt -covermode=atomic

e2e:
	$(GOCMD) test -v ./test/...

.PHONY: unit unit-ci e2e

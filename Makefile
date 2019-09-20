# Go parameters
GOCMD?=go
GOENV=CGO_ENABLED=0

all: dep deps unit

unit:
	@echo "> Running unit tests..."
	$(GOCMD) test -v -count=1 -parallel=1 -timeout=0 ./pkg/...

dep:
ifeq ($(shell command -v dep 2> /dev/null),)
	$(GOCMD) get -u -v github.com/golang/dep/cmd/dep
endif

deps: dep
	dep ensure -v

.PHONY: unit dep deps
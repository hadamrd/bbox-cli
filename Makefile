.PHONY: build test lint vet fmt release-dry-run clean install docs help

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w -X github.com/hadamrd/bbox-cli/cmd.Version=$(VERSION) \
                  -X github.com/hadamrd/bbox-cli/cmd.Commit=$(COMMIT) \
                  -X github.com/hadamrd/bbox-cli/cmd.BuildDate=$(DATE)

help:               ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

build:              ## build local binary (with ldflags)
	go build -ldflags "$(LDFLAGS)" -o bbox$(SUFFIX) .

install:            ## go install with ldflags
	go install -ldflags "$(LDFLAGS)" .

test:               ## run tests with race detector
	go test ./... -race -count=1

lint:               ## run golangci-lint (needs golangci-lint installed)
	golangci-lint run

vet:                ## go vet
	go vet ./...

fmt:                ## gofmt in place
	gofmt -w .

docs:               ## regenerate man pages + markdown docs
	mkdir -p docs/man docs/md
	go run . docs man docs/man
	go run . docs md docs/md

release-dry-run:    ## run goreleaser without publishing (needs goreleaser)
	goreleaser release --snapshot --clean --skip=publish,sign

clean:              ## remove build artifacts
	rm -f bbox bbox.exe
	rm -rf dist/

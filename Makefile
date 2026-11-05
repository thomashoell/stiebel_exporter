EXECUTABLE=wlp_exporter

# if environment variables are defined (e.g. through GitHub Actions), use that. Otherwise, try to resolve them during build
GIT_COMMIT ?= $(shell git rev-list -1 HEAD | cut -c1-7)
GIT_TAG ?= $(shell git describe --tags --always --dirty)

.PHONY: all test fmt lint hooks build build-daedalus snapshot clean

all: build

test:
	go test -race ./...

fmt:
	golangci-lint fmt

lint:
	golangci-lint run

hooks:
	pre-commit install

# builds for the current OS/arch; cross-compile with e.g. `make build GOOS=linux GOARCH=arm64`
build:
	CGO_ENABLED=0 go build -trimpath -o $(EXECUTABLE) -ldflags "-s -w -X main.gitCommit=$(GIT_COMMIT) -X main.gitTag=$(GIT_TAG)"

# Raspberry Pi 3B with 32 bit Debian (armhf)
build-daedalus:
	$(MAKE) build GOOS=linux GOARCH=arm GOARM=7

# builds all release binaries into dist/ without publishing anything
snapshot:
	goreleaser release --snapshot --clean --skip=sbom,sign

clean:
	go clean
	rm -rf dist $(EXECUTABLE)

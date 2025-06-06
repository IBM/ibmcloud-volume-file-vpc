
GOPACKAGES=$(shell go list ./... | grep -v /vendor/ | grep -v /samples | grep -v /common/registry/fakes | grep -v pkg/metadata/fake | grep -v common/vpcclient/client/fakes | grep -v /common/vpcclient/riaas/fakes | grep -v /common/vpcclient/vpcfilevolume/fakes | grep -v /common/vpcclient/riaas/test | grep -v /common/vpcclient/models | grep -v /file/vpcconfig | grep -v /e2e)
GOFILES=$(shell find . -type f -name '*.go' -not -path "./vendor/*")
ARCH = $(shell uname -m)
LINT_VERSION="1.60.1"

.PHONY: all
all: deps dofmt vet test

.PHONY: deps
deps:
	go mod download
	@if ! which golangci-lint >/dev/null || [[ "$$(golangci-lint --version)" != *${LINT_VERSION}* ]]; then \
		curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v${LINT_VERSION}; \
	fi

.PHONY: fmt
fmt:
	golangci-lint run --disable-all --enable=gofmt

.PHONY: dofmt
dofmt:
	golangci-lint run --disable-all --enable=gofmt --fix

.PHONY: lint
lint:
	golangci-lint run

.PHONY: makefmt
makefmt:
	gofmt -l -w ${GOFILES}

.PHONY: build
build:
	go build -gcflags '-N -l' -o libSample samples/main.go samples/volume_operations.go

.PHONY: test
test:
	go test -v -timeout 3000s -coverprofile=cover.out ${GOPACKAGES}

.PHONY: coverage
coverage:
	go tool cover -html=cover.out -o=cover.html
	@./scripts/calculateCoverage.sh

.PHONY: vet
vet:
	go vet ${GOPACKAGES}

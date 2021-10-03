APP                    =genqlient
SHELL                 := sh
.DEFAULT_GOAL         := build
.ONESHELL:
.EXPORT_ALL_VARIABLES:
.SHELLFLAGS           := -eu -o pipefail -c
.DELETE_ON_ERROR:
MAKEFLAGS             += --warn-undefined-variables
MAKEFLAGS             += --no-builtin-rules
SOURCES               ?=$(shell find . -type f -name '*.go' -not -path "./vendor/*") go.mod
GOBUILD               := go build -trimpath
GOINSTALL             := go install -trimpath
COMMIT_SHA            ?=$(shell git rev-parse HEAD)
VERSION                =$(shell go list -f '{{.Version}}' -m github.com/Khan/genqlient@${COMMIT_SHA})
define LDFLAGS
 -X generate.version=${VERSION}
endef

.PHONY: example
example:  ## - Re-generate the example and run the example application
	@printf '\033[32m\xE2\x9c\x93 Re-generating the example and running the example application\n\033[0m'
	go generate ./...
	go run ./example

lint: $(SOURCES) ## - Lint file for problems
	@printf '\033[32m\xE2\x9c\x93 Linting your code\n\033[0m'
	( cd internal/lint && go build -o golangci-lint github.com/golangci/golangci-lint/cmd/golangci-lint )
	internal/lint/golangci-lint run ./... --fix

check: lint $(SOURCES) ## - Check tests, coverage, and tidy go.mod
	@printf '\033[32m\xE2\x9c\x93 Checking your code\n\033[0m'
	go test -cover ./...
	go mod tidy

build: $(SOURCES) ## - Builds genqlient binary
	@printf '\033[32m\xE2\x9c\x93 Building your code\n\033[0m'
	export LDFLAGS
	$(GOBUILD) \
	-a -ldflags "$$LDFLAGS" \
	-o $(APP) ./main.go

.PHONY: install
install: $(SOURCES) ## - Actually installs genqlient
	@printf '\033[32m\xE2\x9c\x93 Building your code\n\033[0m'
	export LDFLAGS
	$(GOINSTALL) \
	-a -ldflags "$$LDFLAGS" \
	./main.go

.PHONY: help
## help: Prints this help message
help: ## - Show help message
	@printf '\033[32m\xE2\x9c\x93 usage: make [target]\n\n\033[0m'
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

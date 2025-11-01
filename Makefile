# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: geth evm all test lint fmt clean devtools help

GOBIN = ./build/bin
GO ?= latest
GORUN = go run

#? geth: Build geth.
geth:
	$(GORUN) build/ci.go install ./cmd/geth
	@echo "Done building."
	@echo "Run \"$(GOBIN)/geth\" to launch geth."

#? evm: Build evm.
evm:
	$(GORUN) build/ci.go install ./cmd/evm
	@echo "Done building."
	@echo "Run \"$(GOBIN)/evm\" to launch evm."

#? all: Build all packages and executables.
all:
	$(GORUN) build/ci.go install

#? test: Run the tests.
test: all
	$(GORUN) build/ci.go test

#? lint: Run certain pre-selected linters.
lint: ## Run linters.
	$(GORUN) build/ci.go lint

#? fmt: Ensure consistent code formatting.
fmt:
	gofmt -s -w $(shell find . -name "*.go")

#? clean: Clean go cache, built executables, and the auto generated folder.
clean:
	go clean -cache
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

#? gethrelay: Build gethrelay.
gethrelay:
	$(GORUN) build/ci.go install ./cmd/gethrelay
	@echo "Done building."
	@echo "Run \"$(GOBIN)/gethrelay\" to launch gethrelay."

#? gethrelay-docker: Build gethrelay Docker image.
gethrelay-docker:
	docker build -f cmd/gethrelay/Dockerfile.gethrelay \
		--build-arg GO_VERSION=1.24 \
		--build-arg COMMIT=$(shell git rev-parse HEAD 2>/dev/null || echo "") \
		--build-arg VERSION=$(shell git describe --tags 2>/dev/null || echo "dev") \
		-t ethereum/gethrelay:latest \
		.

#? gethrelay-test: Run gethrelay unit tests.
gethrelay-test:
	cd cmd/gethrelay && go test -v -race -cover .

#? gethrelay-hive: Run Hive integration tests for gethrelay.
gethrelay-hive:
	@if ! command -v hive >/dev/null 2>&1; then \
		echo "Hive not found. Install with: git clone https://github.com/ethereum/hive && cd hive && go build ."; \
		exit 1; \
	fi
	@bash cmd/gethrelay/test-hive.sh

#? devtools: Install recommended developer tools.
devtools:
	env GOBIN= go install golang.org/x/tools/cmd/stringer@latest
	env GOBIN= go install github.com/fjl/gencodec@latest
	env GOBIN= go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	env GOBIN= go install ./cmd/abigen
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

#? help: Get more info on make commands.
help: Makefile
	@echo ''
	@echo 'Usage:'
	@echo '  make [target]'
	@echo ''
	@echo 'Targets:'
	@sed -n 's/^#?//p' $< | column -t -s ':' |  sort | sed -e 's/^/ /'

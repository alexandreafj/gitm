BINARY     := gitm
CMD        := ./cmd/gitm
BUILD_DIR  := ./bin
INSTALL_DIR := $(shell go env GOPATH)/bin

# Build info
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: all build build-linux install clean test lint lint-check fmt format-check tidy run help

## all: Build the binary (default)
all: build

## build: Compile the gitm binary for macOS into ./bin/
build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD)
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

## build-linux: Cross-compile the gitm binary for Linux amd64 into ./bin/linux/
build-linux:
	@mkdir -p $(BUILD_DIR)/linux
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/linux/$(BINARY) $(CMD)
	@echo "Built: $(BUILD_DIR)/linux/$(BINARY)  (linux/amd64)"

## install: Install gitm to GOPATH/bin (makes it available in your PATH)
install:
	go install $(LDFLAGS) $(CMD)
	@echo "Installed: $(INSTALL_DIR)/$(BINARY)"

## run: Build and run with args (e.g. make run ARGS="repo list")
run: build
	$(BUILD_DIR)/$(BINARY) $(ARGS)

## test: Run all tests with race detection
test:
	go test ./... -v -race -timeout 60s

## lint: Run golangci-lint (auto-fixes where possible)
lint:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run --fix ./... || echo "golangci-lint not installed — run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

## lint-check: Run golangci-lint without fixing (for CI)
lint-check:
	@which golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed — run: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"

## fmt: Format all code with goimports and gofmt
fmt:
	@goimports -w ./internal ./cmd
	@gofmt -s -w ./internal ./cmd

## format-check: Check if code is formatted (for CI)
format-check:
	@if [ -n "$$(gofmt -s -l ./...)" ]; then \
		echo "Code is not formatted. Run: make fmt"; \
		gofmt -s -d ./...; \
		exit 1; \
	fi

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)
	@echo "Cleaned."

## tidy: Tidy go.mod and go.sum
tidy:
	go mod tidy

## help: Show this help message
help:
	@echo "Usage: make [target]"
	@echo ""
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## /  /'

BINARY     := gitm
CMD        := ./cmd/gitm
BUILD_DIR  := ./bin
INSTALL_DIR := $(shell go env GOPATH)/bin

# Build info
VERSION    := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS    := -ldflags "-X main.version=$(VERSION) -s -w"

.PHONY: all build install clean test lint help

## all: Build the binary (default)
all: build

## build: Compile the gitm binary into ./bin/
build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) $(CMD)
	@echo "Built: $(BUILD_DIR)/$(BINARY)"

## install: Install gitm to GOPATH/bin (makes it available in your PATH)
install:
	go install $(LDFLAGS) $(CMD)
	@echo "Installed: $(INSTALL_DIR)/$(BINARY)"

## run: Build and run with args (e.g. make run ARGS="repo list")
run: build
	$(BUILD_DIR)/$(BINARY) $(ARGS)

## test: Run all tests
test:
	go test ./... -v -race -timeout 60s

## lint: Run go vet and staticcheck
lint:
	go vet ./...
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not installed — run: go install honnef.co/go/tools/cmd/staticcheck@latest"

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

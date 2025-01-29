# Makefile for SRTran

# Go settings
GO := go
GO_BUILD := $(GO) build
GO_INSTALL := $(GO) install
GOBIN := $(shell $(GO) env GOPATH)/bin
BINARY_NAME := srtran

# Build settings
BUILD_DIR := build
BINARY_PATH := $(BUILD_DIR)/$(BINARY_NAME)
VERSION := $(shell git describe --tags 2>/dev/null || echo "dev")
BUILD_TIME := $(shell date +%FT%T%z)
GIT_COMMIT := $(shell git rev-parse --short HEAD)

# Linker flags
LDFLAGS := -ldflags "-X github.com/s0up4200/SRTran/cmd.Version=${VERSION} \
	-X github.com/s0up4200/SRTran/cmd.BuildTime=${BUILD_TIME} \
	-X github.com/s0up4200/SRTran/cmd.GitCommit=${GIT_COMMIT}"

# Default target
.DEFAULT_GOAL := build

# Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) $(LDFLAGS) -o $(BINARY_PATH) ./cmd

# Install the binary to $GOPATH/bin
install:
	$(GO_INSTALL) $(LDFLAGS) ./...

# Clean build directory
clean:
	@rm -rf $(BUILD_DIR)

# Run the program
run: build
	$(BINARY_PATH)

.PHONY: build install clean run
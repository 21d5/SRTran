# Makefile for SRTran

# Go settings
GO := go
GO_BUILD := $(GO) build
GO_INSTALL := $(GO) install
BINARY_NAME := srtran

# Build settings
BUILD_DIR := build
BINARY_PATH := $(BUILD_DIR)/$(BINARY_NAME)

# Default target
.DEFAULT_GOAL := build

# Build the binary
build:
	@mkdir -p $(BUILD_DIR)
	$(GO_BUILD) -o $(BINARY_PATH) ./cmd

# Install the binary to $GOPATH/bin
install:
	$(GO_INSTALL) ./

# Clean build directory
clean:
	@rm -rf $(BUILD_DIR)

# Run the program
run: build
	$(BINARY_PATH)

.PHONY: build install clean run
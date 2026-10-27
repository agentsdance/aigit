BINARY_NAME=aigit
VERSION=$(shell git describe --tags --always --dirty)

# Build directories
BUILD_DIR=build
LINUX_AMD64=$(BUILD_DIR)/$(BINARY_NAME)_linux_amd64_$(VERSION)
MACOS_AMD64=$(BUILD_DIR)/$(BINARY_NAME)_darwin_amd64_$(VERSION)
MACOS_ARM64=$(BUILD_DIR)/$(BINARY_NAME)_darwin_arm64_$(VERSION)
WINDOWS_AMD64=$(BUILD_DIR)/$(BINARY_NAME)_windows_amd64_$(VERSION).exe

.PHONY: all clean build-all build-linux build-macos-amd64 build-macos-arm64 build-windows

all: build-all

build-all: clean linux macos-amd64 macos-arm64 windows
	@echo "Build complete! Binaries are in the $(BUILD_DIR) directory"

linux:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(LINUX_AMD64) .

macos-amd64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(MACOS_AMD64) .

macos-arm64:
	@mkdir -p $(BUILD_DIR)
	GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(MACOS_ARM64) .

windows:
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X main.Version=$(VERSION)" -o $(WINDOWS_AMD64) .

clean:
	@rm -rf $(BUILD_DIR)

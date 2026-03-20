BINARY_NAME := dns-updater
VERSION := $(shell date -u +%Y%m%d.%H%M).$(shell git rev-parse --short HEAD)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"
OUT_DIR := dist/bin

.PHONY: build build-all clean release

$(OUT_DIR):
	mkdir -p $(OUT_DIR)

build: $(OUT_DIR)
	go build $(LDFLAGS) -o $(OUT_DIR)/$(BINARY_NAME) .

build-windows: $(OUT_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(OUT_DIR)/$(BINARY_NAME)-windows-amd64.exe .

build-linux: $(OUT_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(OUT_DIR)/$(BINARY_NAME)-linux-amd64 .

build-linux-armv7l: $(OUT_DIR)
	GOOS=linux GOARCH=arm GOARM=7 go build $(LDFLAGS) -o $(OUT_DIR)/$(BINARY_NAME)-linux-armv7l .

build-darwin: $(OUT_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(OUT_DIR)/$(BINARY_NAME)-darwin-amd64 .

build-darwin-arm64: $(OUT_DIR)
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(OUT_DIR)/$(BINARY_NAME)-darwin-arm64 .

build-all: build-windows build-linux build-linux-armv7l build-darwin build-darwin-arm64

release:
	./build-release.sh

clean:
	rm -rf dist/

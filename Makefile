BINARY_NAME := dns-updater
VERSION := $(shell date -u +%Y%m%d.%H%M).$(shell git rev-parse --short HEAD)
LDFLAGS := -ldflags "-X main.Version=$(VERSION)"

.PHONY: build build-all clean release

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) .

build-windows:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME).exe .

build-linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-linux .

build-darwin:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin .

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .

build-all: build-windows build-linux build-darwin build-darwin-arm64

release:
	./build-release.sh

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe $(BINARY_NAME)-linux $(BINARY_NAME)-darwin $(BINARY_NAME)-darwin-arm64 checksums.txt

MODULE := github.com/kyungw00k/dbibackend
BINARY := dbibackend
BUILD_DIR := dist

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X $(MODULE)/cmd.Version=$(VERSION)"

.PHONY: build install clean run run-cli test lint snapshot

build:
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/$(BINARY)

install: build
	mkdir -p $(HOME)/.local/bin
	cp $(BUILD_DIR)/$(BINARY) $(HOME)/.local/bin/$(BINARY)

clean:
	rm -rf $(BUILD_DIR)

run: build
	$(BUILD_DIR)/$(BINARY)

run-cli: build
	$(BUILD_DIR)/$(BINARY) --cli

test:
	go test ./...

lint:
	go vet ./...

snapshot:
	goreleaser release --snapshot --clean

.PHONY: all build test test-e2e fmt vet install clean

VERSION ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"
INSTALL_DIR ?= $(HOME)/.local/bin

all: fmt vet test build

build:
	go build $(LDFLAGS) -o logfire-mcp ./cmd/logfire-mcp
	go build $(LDFLAGS) -o logfire-cli ./cmd/logfire-cli

test:
	go test -v ./...

test-e2e: build
	@if command -v uv >/dev/null 2>&1; then \
		uv run tools/test_mcp.py; \
	else \
		python3 tools/test_mcp.py; \
	fi

fmt:
	go fmt ./...

vet:
	go vet ./...

install:
	mkdir -p $(INSTALL_DIR)
	go build $(LDFLAGS) -o $(INSTALL_DIR)/logfire-mcp ./cmd/logfire-mcp
	go build $(LDFLAGS) -o $(INSTALL_DIR)/logfire-cli ./cmd/logfire-cli

clean:
	rm -f logfire-mcp logfire-cli

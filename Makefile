BINARY := orb
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)
INSTALL_DIR := /usr/local/bin

.PHONY: build install uninstall test lint fmt demo clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/$(BINARY)/

install: build
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)

test:
	go test ./... -count=1 -race

lint:
	go vet ./...
	@command -v staticcheck >/dev/null && staticcheck ./... || echo "staticcheck not installed, skipped"

fmt:
	gofmt -s -w .

demo: build
	./$(BINARY) -demo

clean:
	rm -f $(BINARY)

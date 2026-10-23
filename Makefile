BINARY := orb
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)
INSTALL_DIR := /usr/local/bin

.PHONY: build install uninstall test test-priv lint fmt demo clean

# CGO off: a fully static binary, no libc, runs on any Linux box as is.
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/$(BINARY)/

install: build
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)

test:
	go test ./... -count=1 -race

# The privilege drop for real: a static test binary run as root of a user
# namespace (needs newuidmap and a /etc/subuid range for you).
test-priv:
	@d=$$(mktemp -d) && CGO_ENABLED=0 go test -c -o $$d/priv.test ./internal/priv && \
	s=0; for t in TestDropHelperCaps TestDropMainNoCaps TestDropMainTTYConfig; do \
		ORB_PRIV_TEST=1 unshare --map-auto --map-root-user $$d/priv.test -test.v -test.run "^$$t$$" || s=1; \
	done; \
	ORB_PRIV_TEST=1 unshare --map-auto --map-root-user --net $$d/priv.test -test.v -test.run '^TestLLDPHelperDrop$$' || s=1; \
	CGO_ENABLED=0 go build -o $$d/orb ./cmd/orb && \
	unshare --map-auto --map-root-user scripts/test-privsep.sh $$d/orb || s=1; rm -rf $$d; exit $$s

lint:
	go vet ./...
	@command -v staticcheck >/dev/null && staticcheck ./... || echo "staticcheck not installed, skipped"

fmt:
	gofmt -s -w .

demo: build
	./$(BINARY) -demo

clean:
	rm -f $(BINARY)

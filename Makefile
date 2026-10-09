GO ?= go
GOFLAGS ?=

.PHONY: all build test test-js vet fmt clean

all: vet test build

build:
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o /tmp/deploy-dashboard-static ./cmd/

test:
	$(GO) test -v -count=1 ./cmd/

test-js:
	node web/test.js

vet:
	$(GO) vet ./cmd/

fmt:
	$(GO) fmt ./cmd/

clean:
	rm -f /tmp/deploy-dashboard-static

GO ?= go
GOFLAGS ?=

.PHONY: all build test test-js vet fmt check-embed clean

all: vet test check-embed build

build:
	CGO_ENABLED=0 $(GO) build -ldflags="-s -w" -o /tmp/deploy-dashboard-static ./cmd/

test:
	$(GO) test -v -count=1 ./cmd/

check-embed: build
	@strings /tmp/deploy-dashboard-static | grep -q "Devpod" \
	  || { echo "FAIL: binary does not embed the current web/index.html"; exit 1; }
	@echo "OK: embedded UI matches web/index.html"

test-js:
	node web/test.js

vet:
	$(GO) vet ./cmd/

fmt:
	$(GO) fmt ./cmd/

clean:
	rm -f /tmp/deploy-dashboard-static

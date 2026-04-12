GO = $(shell which go 2>/dev/null)

APP             := overwatcher
AGENT           := agent
VERSION         ?= v0.1.0
LDFLAGS         := -ldflags "-X main.AppVersion=$(VERSION)"

.PHONY: all build build-agent clean run run-agent test

all: clean build build-agent

clean:
	$(GO) clean -testcache
	$(RM) -rf bin/*
build:
	$(GO) build -o bin/$(APP) $(LDFLAGS) cmd/$(APP)/*.go
build-agent:
	$(GO) build -o bin/$(AGENT) $(LDFLAGS) cmd/$(AGENT)/*.go
run:
	$(GO) run $(LDFLAGS) cmd/$(APP)/*.go
run-agent:
	$(GO) run $(LDFLAGS) cmd/$(AGENT)/*.go
test:
	$(GO) test -v ./...

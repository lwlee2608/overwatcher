GO = $(shell which go 2>/dev/null)
DOCKER = $(shell which docker 2>/dev/null)

REGISTRY        ?= lwlee2608/
APP             := overwatcher
AGENT           := agent
VERSION         ?= v0.2.0
LDFLAGS         := -ldflags "-X main.AppVersion=$(VERSION)"

.PHONY: all build build-agent clean run run-agent test dep generate

all: clean build build-agent

dep:
	$(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0
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
generate:
	sqlc generate

docker:
	$(DOCKER) build --build-arg APP=$(APP) --build-arg VERSION=$(VERSION) -t $(REGISTRY)$(APP):$(VERSION) -t $(REGISTRY)$(APP):latest .
docker-agent:
	$(DOCKER) build -f Dockerfile.agent --build-arg VERSION=$(VERSION) -t $(REGISTRY)$(AGENT):$(VERSION) -t $(REGISTRY)$(AGENT):latest .
push:
	$(DOCKER) push $(REGISTRY)$(APP):latest
push-agent:
	$(DOCKER) push $(REGISTRY)$(AGENT):latest

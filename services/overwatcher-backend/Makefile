GO = $(shell which go 2>/dev/null)
DOCKER = $(shell which docker 2>/dev/null)

REGISTRY        ?= lwlee2608/
APP             := overwatcher
AGENT           := agent
VERSION         ?= v0.3.0
LDFLAGS         := -ldflags "-X main.AppVersion=$(VERSION)"

.PHONY: all build build-agent clean run run-agent test dep generate release-agent

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
	$(DOCKER) push $(REGISTRY)$(APP):$(VERSION)
	$(DOCKER) push $(REGISTRY)$(APP):latest
push-agent:
	$(DOCKER) push $(REGISTRY)$(AGENT):$(VERSION)
	$(DOCKER) push $(REGISTRY)$(AGENT):latest

# release-agent cross-compiles the systemd agent binary for linux/amd64 and
# linux/arm64, strips it (`-s -w`), and writes a SHA256SUMS file. Output goes
# to dist/. The release workflow uploads this dir to a GitHub release.
release-agent:
	$(RM) -rf dist
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
		$(GO) build -trimpath -ldflags "-s -w -X main.AppVersion=$(VERSION)" \
		-o dist/overwatcher-agent_linux_amd64 cmd/$(AGENT)/*.go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
		$(GO) build -trimpath -ldflags "-s -w -X main.AppVersion=$(VERSION)" \
		-o dist/overwatcher-agent_linux_arm64 cmd/$(AGENT)/*.go
	cd dist && shasum -a 256 overwatcher-agent_linux_amd64 overwatcher-agent_linux_arm64 > SHA256SUMS

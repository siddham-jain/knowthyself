.PHONY: build install uninstall dev sandbox which test lint tidy clean release-check

# Version metadata is stamped into the binary so `knowthyself --version` always says
# exactly which build you are running. `git describe --dirty` makes a local build
# obviously different from a release (v0.3.2-2-g62f95d7-dirty vs 0.3.2).
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# ~/.local/bin comes before the npm and Homebrew directories on PATH, so installing
# here shadows the released binary and uninstalling brings it straight back.
BINDIR  ?= $(HOME)/.local/bin

# A throwaway config for testing. Keeping it out of ~/.config means an experiment can
# never touch your real providers or API keys.
SANDBOX ?= $(CURDIR)/.sandbox

build:
	go build -ldflags "$(LDFLAGS)" -o knowthyself ./cmd/knowthyself

## install: put this build on your PATH, shadowing the released one
install: build
	@mkdir -p $(BINDIR)
	@install -m 0755 knowthyself $(BINDIR)/knowthyself
	@echo "installed $(VERSION) to $(BINDIR)/knowthyself"
	@echo "it shadows the released binary until you run: make uninstall"

## uninstall: remove the local build, restoring the released binary
uninstall:
	@rm -f $(BINDIR)/knowthyself
	@echo "removed $(BINDIR)/knowthyself"
	@command -v knowthyself >/dev/null 2>&1 \
		&& echo "back to: $$(command -v knowthyself) ($$(knowthyself --version 2>/dev/null | head -1))" \
		|| echo "no knowthyself left on PATH"

## dev: run this build against your real config    e.g. make dev ARGS="--deep-eval"
dev: build
	@./knowthyself $(ARGS)

## sandbox: run this build against a throwaway config in .sandbox/
## Nothing here can touch your real providers or keys.
##   make sandbox ARGS="provider add openrouter --key-env OPENROUTER_API_KEY"
##   make sandbox ARGS="--deep-eval"
sandbox: build
	@mkdir -p $(SANDBOX)
	@XDG_CONFIG_HOME=$(SANDBOX) ./knowthyself $(ARGS)

## which: show every knowthyself on your PATH, in precedence order
which:
	@echo "$$PATH" | tr ':' '\n' | awk '!seen[$$0]++' | while read -r d; do \
		if [ -x "$$d/knowthyself" ]; then \
			printf '%-52s %s\n' "$$d/knowthyself" "$$("$$d/knowthyself" --version 2>/dev/null | head -1)"; \
		fi; \
	done
	@printf '\nactive: %s\n' "$$(command -v knowthyself 2>/dev/null || echo none)"

test:
	go test ./...

lint:
	go vet ./...
	@test -z "$$(gofmt -l . | grep -v '^$$')" || (echo "gofmt needed:"; gofmt -l .; exit 1)

tidy:
	go mod tidy

## release-check: everything CI would check, before you tag
release-check: lint test build
	@echo "ok — safe to tag"

clean:
	rm -f knowthyself
	rm -rf dist $(SANDBOX)

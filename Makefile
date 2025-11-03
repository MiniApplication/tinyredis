SHELL := /bin/bash

# --- Toolchain configuration -------------------------------------------------
GO        ?= go
GOFMT     ?= gofmt
GOENV      = GOCACHE=$(CURDIR)/.cache/go-build GOMODCACHE=$(CURDIR)/.cache/go-mod
GOFLAGS   ?=
MODULE_CACHE_SENTINEL := $(CURDIR)/.cache/go-mod/.synced

# --- Project layout ----------------------------------------------------------
BINARY    := tinyredis
PKG_MAIN  := ./cmd/tinyredis
BUILD_DIR := bin
BIN       := $(BUILD_DIR)/$(BINARY)
PKGS      := ./...

GOFILES := $(shell find . -name '*.go' -not -path './vendor/*' -not -path './.cache/*')

# --- Quality tooling ---------------------------------------------------------
GOTEST_FLAGS ?= -race -timeout 60s
BENCH_FLAGS  ?= -run=^$$ -bench=.
LINT_CMD     ?= golangci-lint
LINT_ARGS    ?= run ./...
LINT_VERSION ?= v1.61.0

# --- Docker settings ---------------------------------------------------------
IMAGE     ?= tinyredis:latest
CONTAINER ?= tinyredis
DATA_DIR  ?= $(CURDIR)/data

# --- Help --------------------------------------------------------------------
.PHONY: help
help: ## Show this help
	@printf "Usage: make <target>\n\nTargets:\n"
	@grep -E '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | awk 'BEGIN {FS=":.*##"} {printf "  %-20s %s\n", $$1, $$2}'

# --- Build & Run -------------------------------------------------------------
.PHONY: build
build: ## Build the tinyredis binary
	@rm -rf $(BUILD_DIR)
	@mkdir -p $(BUILD_DIR)
	@if [ ! -f $(MODULE_CACHE_SENTINEL) ]; then \
		echo "==> priming Go module cache"; \
		rm -rf $(CURDIR)/.cache/go-mod; \
		mkdir -p $(CURDIR)/.cache/go-mod; \
		cp -a $$HOME/go/pkg/mod/. $(CURDIR)/.cache/go-mod >/dev/null 2>&1 || true; \
		touch $(MODULE_CACHE_SENTINEL); \
	fi
	@echo "==> building $(BIN)"
	@$(GOENV) $(GO) build $(GOFLAGS) -o $(BIN) $(PKG_MAIN)

.PHONY: run
run: build ## Run previously built binary
	@$(BIN)

.PHONY: run-dev
run-dev: ## Run tinyredis via go run (no build artifacts)
	@$(GOENV) $(GO) run $(GOFLAGS) $(PKG_MAIN)

# --- Testing & Verification --------------------------------------------------
.PHONY: test
test: ## Run unit tests with race detector
	@$(GOENV) $(GO) test $(GOFLAGS) $(GOTEST_FLAGS) $(PKGS)

.PHONY: test-short
test-short: ## Run short tests without race detector
	@$(GOENV) $(GO) test $(GOFLAGS) -short $(PKGS)

.PHONY: bench
bench: ## Run benchmarks for the project
	@$(GOENV) $(GO) test $(GOFLAGS) $(BENCH_FLAGS) $(PKGS)

.PHONY: test-leader-transfer
test-leader-transfer: ## Run the Raft leader failover integration test
	@$(GOENV) $(GO) test $(GOFLAGS) ./pkg/cluster -run TestLeaderFailoverTransfersLeadership -count=1

.PHONY: lint
lint: ## Run golangci-lint (installs if missing)
	@if ! command -v $(LINT_CMD) >/dev/null 2>&1; then \
		echo "Installing $(LINT_CMD) $(LINT_VERSION)"; \
		$(GOENV) $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(LINT_VERSION); \
	fi
	@$(GOENV) $(LINT_CMD) $(LINT_ARGS)

.PHONY: fmt
fmt: ## Format Go source files in-place
	@$(GOFMT) -w $(GOFILES)

.PHONY: fmt-check
fmt-check: ## Check that Go files are formatted
	@fmt_out=$$($(GOFMT) -l $(GOFILES)); \
	if [ -n "$$fmt_out" ]; then \
		echo "Go files need formatting:"; \
		echo "$$fmt_out"; \
		exit 1; \
	fi

.PHONY: tidy
tidy: ## Run go mod tidy
	@$(GOENV) $(GO) mod tidy

.PHONY: vet
vet: ## Run go vet on the codebase
	@$(GOENV) $(GO) vet $(PKGS)

.PHONY: ci
ci: fmt-check lint test ## Run formatting check, lint, and tests

# --- Docker ------------------------------------------------------------------
.PHONY: docker-build
docker-build: ## Build docker image
	@docker build -t $(IMAGE) .

.PHONY: docker-run
docker-run: docker-build ## Run docker container locally
	@docker run -d --rm --name $(CONTAINER) \
		-p 6379:6379 \
		-v $(DATA_DIR)/node1:/data \
		$(IMAGE) \
		./tinyredis --host 0.0.0.0 --node-id node-1 \
		--raft-dir /data --raft-bind 0.0.0.0:7000 --raft-http 0.0.0.0:17000 --raft-bootstrap

.PHONY: docker-stop
docker-stop: ## Stop the running docker container (if present)
	@docker stop $(CONTAINER) >/dev/null 2>&1 || true

.PHONY: docker-shell
docker-shell: ## Open an interactive shell inside the container
	@docker exec -it $(CONTAINER) /bin/sh

# --- Convenience targets -----------------------------------------------------
BOOTSTRAP_HOST ?= 127.0.0.1
BOOTSTRAP_PORT ?= 6379
BOOTSTRAP_NODE ?= node-1
BOOTSTRAP_DIR  ?= $(CURDIR)/data/$(BOOTSTRAP_NODE)
BOOTSTRAP_RAFT ?= 127.0.0.1:7000
BOOTSTRAP_HTTP ?= 127.0.0.1:17000

.PHONY: bootstrap
bootstrap: build ## Start a bootstrap node with sensible defaults
	@mkdir -p $(BOOTSTRAP_DIR)
	@$(BIN) \
		--host $(BOOTSTRAP_HOST) --port $(BOOTSTRAP_PORT) \
		--node-id $(BOOTSTRAP_NODE) \
		--raft-dir $(BOOTSTRAP_DIR) \
		--raft-bind $(BOOTSTRAP_RAFT) \
		--raft-http $(BOOTSTRAP_HTTP) \
		--raft-bootstrap

JOIN_NODE   ?= node-2
JOIN_PORT   ?= 6380
JOIN_RAFT   ?= 127.0.0.1:7001
JOIN_HTTP   ?= 127.0.0.1:17001
JOIN_DIR    ?= $(CURDIR)/data/$(JOIN_NODE)
JOIN_TARGET ?= 127.0.0.1:17000

.PHONY: join
join: build ## Start a follower node and join an existing leader
	@mkdir -p $(JOIN_DIR)
	@$(BIN) \
		--host 127.0.0.1 --port $(JOIN_PORT) \
		--node-id $(JOIN_NODE) \
		--raft-dir $(JOIN_DIR) \
		--raft-bind $(JOIN_RAFT) \
		--raft-http $(JOIN_HTTP) \
		--raft-join $(JOIN_TARGET)

# --- Cleanup -----------------------------------------------------------------
.PHONY: clean
clean: ## Remove build artifacts
	@rm -rf $(BUILD_DIR)

.PHONY: clean-cache
clean-cache: ## Remove Go build cache used by make
	@rm -rf .cache

.PHONY: clean-all
clean-all: clean clean-cache ## Remove build artifacts and Go caches

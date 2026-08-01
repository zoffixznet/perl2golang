GO      ?= go
BINDIR  ?= $(HOME)/.local/bin
BIN     := perl2go

.DEFAULT_GOAL := help

.PHONY: help build install test test-short score lint fmt vet clean run explain repl repl-demo demo deps corpus-add

help: ## Show this help
	@echo "perl2go - convert Perl 5 scripts to Go and learn Go along the way"
	@echo
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'
	@echo
	@echo "make install puts the binary in $(BINDIR); set BINDIR to change that."

build: ## Build the perl2go binary into ./bin
	$(GO) build -o bin/$(BIN) ./cmd/perl2go

install: ## Install perl2go where BINDIR says (see below)
	$(GO) build -o $(BINDIR)/$(BIN) ./cmd/perl2go
	@echo "installed $(BINDIR)/$(BIN)"

test: ## Run the full test suite
	$(GO) test ./...

test-short: ## Run the quick tests only (skips toolchain-heavy tests)
	$(GO) test -short ./...

score: ## Score the conversion over the corpus: ARGS='-tier tier2 -v' narrows it
	$(GO) run ./cmd/score $(ARGS)

lint: vet ## Run static checks (alias for vet plus gofmt check)
	@fmtout=$$(gofmt -l . 2>/dev/null); if [ -n "$$fmtout" ]; then echo "gofmt needed on:"; echo "$$fmtout"; exit 1; fi

fmt: ## Format all Go sources
	gofmt -w .

vet: ## Run go vet over all packages
	$(GO) vet ./...

clean: ## Remove build artifacts
	$(GO) clean ./...
	rm -rf bin

run: build ## Build and show top-level help
	./bin/$(BIN) --help

explain: build ## List the teaching concepts, or read one: make explain TOPIC=nil-vs-undef
	@if [ -z "$(TOPIC)" ]; then ./bin/$(BIN) explain --list; else ./bin/$(BIN) explain "$(TOPIC)"; fi

repl: build ## Start the interactive session: type Perl, see the Go
	./bin/$(BIN) repl

repl-demo: build ## Pipe a canned session through the repl and check the transcript
	./scripts/repl-demo.sh ./bin/$(BIN)

demo: build ## Convert a sample corpus script into a temp directory and run it
	./scripts/demo.sh

deps: ## Check for required system tools and offer to install missing ones
	./scripts/deps.sh

corpus-add: ## Scaffold a new corpus entry: make corpus-add TIER=tier2 NAME=my-case
	./scripts/corpus-add.sh $(TIER) $(NAME)

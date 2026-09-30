# Host operator surface: should-i-read CLI (Pimalaya default) + Polypus + optional EmailOps legacy

BIN := bin/should-i-read
EMAILOPS_DIR := providers/emailops
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/xynova/should-i-read/internal/cli.Version=$(VERSION)

.PHONY: help build test tidy check-polypus polypus-check \
	emailops-submodule emailops-install emailops-cli \
	doctor sync export version init setup \
	emailops-doctor emailops-sync emailops-export emailops-accounts emailops-emails emailops-ui \
	pim-doctor pim-sync pim-snapshot wire-ai-copilots bootstrap-ai-copilots

help:
	@echo "Pimalaya (default operator path):"
	@echo "  init                Create ~/.config/should-i-read/config.yaml + data dir"
	@echo "  setup               OAuth client credentials (Gmail/Outlook apps)"
	@echo "  build               Build $(BIN)"
	@echo "  doctor              should-i-read pim doctor (Neverest check)"
	@echo "  sync                should-i-read pim sync"
	@echo "  export              should-i-read pim snapshot"
	@echo "  polypus-check       should-i-read polypus check"
	@echo "  check-polypus       polypus check + scripts/check-polypus.sh"
	@echo "  wire-ai-copilots    Symlink .cursor/skills (see docs/ai-copilots-setup.md)"
	@echo ""
	@echo "Other:"
	@echo "  test                go test ./..."
	@echo "  tidy                go mod tidy"
	@echo "  version             Print CLI version"
	@echo ""
	@echo "EmailOps legacy (submodule; optional custody):"
	@echo "  emailops-ui         Launch EmailOps desktop (make dev)"
	@echo "  emailops-doctor     emailops-cli doctor"
	@echo "  emailops-sync       emailops-cli sync"
	@echo "  emailops-export     Host mail export JSON"
	@echo "  emailops-accounts   List EmailOps accounts"
	@echo "  emailops-emails     List messages via EmailOps"
	@echo "  emailops-submodule  Init/update providers/emailops"
	@echo "  emailops-install    npm install inside EmailOps submodule"
	@echo "  emailops-cli        Build emailops-cli without llama.cpp"

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/should-i-read

test:
	go test ./...

tidy:
	go mod tidy

version: build
	./$(BIN) version

init: build
	./$(BIN) init

setup: build
	./$(BIN) setup

# Default lane aliases (Neverest + pimdir)
doctor: pim-doctor

sync: pim-sync

export: pim-snapshot

pim-doctor: build
	./$(BIN) pim doctor

pim-sync: build
	./$(BIN) pim sync $(ARGS)

pim-snapshot: build
	./$(BIN) pim snapshot $(ARGS)

polypus-check: build
	./$(BIN) polypus check

check-polypus: build
	./$(BIN) polypus check
	./scripts/check-polypus.sh

# EmailOps legacy targets
emailops-ui: build
	./$(BIN) ui

emailops-doctor: build
	./$(BIN) doctor

emailops-accounts: build
	./$(BIN) accounts

emailops-sync: build
	./$(BIN) sync $(ARGS)

emailops-emails: build
	./$(BIN) emails $(ARGS)

emailops-export: build
	./$(BIN) export $(ARGS)

emailops-submodule:
	git submodule update --init --recursive $(EMAILOPS_DIR)

emailops-install: emailops-submodule
	$(MAKE) -C $(EMAILOPS_DIR) install

emailops-cli: emailops-submodule
	cd $(EMAILOPS_DIR) && cargo build \
		--manifest-path src-tauri/Cargo.toml \
		--target-dir src-tauri/target \
		--no-default-features \
		--features cli \
		--bin emailops-cli

wire-ai-copilots:
	bash scripts/wire-cursor-skills.sh

bootstrap-ai-copilots: wire-ai-copilots

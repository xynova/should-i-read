# Host operator surface: should-i-read CLI + EmailOps (black box) + Polypus

BIN := bin/should-i-read
EMAILOPS_DIR := providers/emailops
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/xynova/should-i-read/internal/cli.Version=$(VERSION)

.PHONY: help build test tidy check-polypus polypus-check \
	emailops-submodule emailops-install emailops-cli \
	doctor accounts sync emails export version init setup ui

help:
	@echo "Targets:"
	@echo "  build               Build $(BIN)"
	@echo "  test                go test ./..."
	@echo "  tidy                go mod tidy"
	@echo "  init                Create ~/.config/should-i-read/config.yaml + data dir"
	@echo "  setup               Interactive mail OAuth setup (product / BYO / guided DIY)"
	@echo "  ui                  Launch EmailOps desktop with host config env"
	@echo "  version             Print CLI version"
	@echo "  doctor              should-i-read doctor"
	@echo "  accounts            should-i-read accounts"
	@echo "  sync                should-i-read sync"
	@echo "  emails              should-i-read emails"
	@echo "  export              should-i-read export"
	@echo "  polypus-check       should-i-read polypus check"
	@echo "  check-polypus       Alias for polypus-check (also runs scripts/check-polypus.sh)"
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

ui: build
	./$(BIN) ui

doctor: build
	./$(BIN) doctor

accounts: build
	./$(BIN) accounts

sync: build
	./$(BIN) sync $(ARGS)

emails: build
	./$(BIN) emails $(ARGS)

export: build
	./$(BIN) export $(ARGS)

polypus-check: build
	./$(BIN) polypus check

check-polypus: polypus-check
	./scripts/check-polypus.sh

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

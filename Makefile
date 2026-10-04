# Host operator surface: should-i-read CLI (Pimalaya mail + Polypus)

BIN := bin/should-i-read
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/xynova/should-i-read/internal/cli.Version=$(VERSION)

.PHONY: help build test tidy check-polypus polypus-check \
	readiness doctor sync export version init setup mail-setup mail-status ensure configure \
	mail-readiness mail-sync mail-export pim-ensure pim-deps-check wire-ai-copilots bootstrap-ai-copilots

help:
	@echo "Pimalaya (default operator path):"
	@echo "  init                Create ~/.config/should-i-read/config.yaml"
	@echo "  build               Build $(BIN)"
	@echo "  configure           should-i-read configure $(ARGS) (operator onboarding hub)"
	@echo "  mail-status         should-i-read mail status"
	@echo "  readiness           mail-deps check + should-i-read mail readiness"
	@echo "  doctor              alias for readiness"
	@echo "  sync                mail-deps check + should-i-read mail sync"
	@echo "  export              should-i-read mail export"
	@echo "  polypus-check       should-i-read polypus check"
	@echo "  check-polypus       polypus check + scripts/check-polypus.sh"
	@echo "  wire-ai-copilots    Symlink .cursor/skills (see docs/ai-copilots-setup.md)"
	@echo ""
	@echo "Other:"
	@echo "  test                go test ./..."
	@echo "  tidy                go mod tidy"
	@echo "  version             Print CLI version"

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

mail-setup: build
	./$(BIN) mail setup $(ARGS)

mail-status: build
	./$(BIN) mail status

# Default lane (host CLI; mail sync binary is an implementation detail)
readiness: mail-readiness

doctor: readiness

sync: mail-sync

export: mail-export

ensure: pim-ensure

pim-ensure: build
	./$(BIN) pim ensure

configure: build
	./$(BIN) configure $(ARGS)

pim-configure: build
	./$(BIN) pim configure $(ARGS)

# Internal preflight for readiness/sync (not listed as a Neverest operator verb).
pim-deps-check:
	./scripts/check-neverest.sh

mail-readiness: build pim-deps-check
	./$(BIN) mail readiness

mail-sync: build pim-deps-check
	./$(BIN) mail sync $(ARGS)

mail-export: build
	./$(BIN) mail export $(ARGS)

polypus-check: build
	./$(BIN) polypus check

check-polypus: build
	./$(BIN) polypus check
	./scripts/check-polypus.sh

wire-ai-copilots:
	bash scripts/wire-cursor-skills.sh

bootstrap-ai-copilots: wire-ai-copilots

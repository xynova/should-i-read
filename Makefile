# Host helpers for EmailOps + Polypus

.PHONY: help check-polypus emailops-doctor emailops-submodule

help:
	@echo "Targets:"
	@echo "  check-polypus       Probe Polypus /health and /v1/models (fail closed)"
	@echo "  emailops-submodule  Init/update providers/emailops"
	@echo "  emailops-doctor     Run emailops-cli doctor --json (build cli-fast)"

check-polypus:
	./scripts/check-polypus.sh

emailops-submodule:
	git submodule update --init --recursive providers/emailops

emailops-doctor: emailops-submodule
	$(MAKE) -C providers/emailops cli-fast ARGS="doctor --json"

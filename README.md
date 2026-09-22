# should-i-read

Host project for local inbox triage: EmailOps for mail, Polypus as the only AI gateway, and a later Jev decision path for unwanted-mail filtering.

## Layout

| Path | Role |
|------|------|
| [`providers/emailops`](providers/emailops) | Git submodule: [emailops/emailops](https://github.com/emailops/emailops) desktop client and CLI |
| [`docs/`](docs/) | Host-owned setup, provider seam, and report-only evaluation contract |
| [`config/`](config/) | Example env for Polypus-only AI wiring (no secrets) |

Polypus itself lives outside this tree (typical checkout: `~/Xynova/ai/polypus`). Clients call only `http://127.0.0.1:1320`.

## Quick start

1. Initialize submodules (after clone):

   ```bash
   git submodule update --init --recursive
   ```

2. Start Polypus from its repo (`make serve`). Confirm health:

   ```bash
   ./scripts/check-polypus.sh
   ```

3. Follow [docs/emailops-setup.md](docs/emailops-setup.md) for EmailOps prerequisites, OAuth, and CLI.

4. Read [docs/ai-provider-seam.md](docs/ai-provider-seam.md) before pointing EmailOps at any model. Embedded llama.cpp and direct OpenRouter are not allowed for this host.

5. Keep the first unwanted-mail milestone report-only: [docs/report-only-eval.md](docs/report-only-eval.md).

## Milestone boundary

This host currently pins EmailOps and documents the Polypus-only contract. Jev classification, clustering, and any mailbox mutation are follow-ups after the provider adapter lands.

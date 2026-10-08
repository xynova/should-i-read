# Pimalaya ecosystem AI index

Host-owned context pack for designing should-i-read mail storage and CLI orchestration. Upstream map: [pimalaya.org/ecosystem](https://pimalaya.org/ecosystem). Local evidence: shallow clones under `tmp/pimalaya/` (gitignored).

**Load first:** requestable rule `.cursor/rules/pimalaya-ecosystem.mdc` and skill `ai-copilots/skills/pimalaya-ecosystem/SKILL.md`.

**Product architecture draft:** [architecture.md](architecture.md). **Go host tool contract:** [integration.md](integration.md).

## How to use this pack

1. Read this file for status legend and navigation.
2. Open cards in `store/`, `apps/`, and `libs/` for capabilities and host fit.
3. Verify claims against `tmp/pimalaya/<repo>/README.md` when clones exist.
4. Do not treat third-party mail desktop apps as the target mail platform; design around pimdir + Pimalaya CLIs.

## Status legend

| Status | Meaning |
|--------|---------|
| stable | Ecosystem page: safe to document as dependable |
| beta | Usable; breaking changes possible (Neverest, Carillon note `v0.x`) |
| early | API or backends still moving |
| in_development | No stable release story yet |
| retiring | Mirador; migrate mental model to Carillon |
| frozen / deprecated | See [frozen/INDEX.md](frozen/INDEX.md) |

## Domain map

| Domain | Store / spec | Ingest | Query / mutate remote | Auth | Watch | Convert |
|--------|--------------|--------|------------------------|------|-------|---------|
| Email | [pimdir](store/pimdir.md), [io-pimdir](store/io-pimdir.md) | [neverest](apps/neverest.md) | [himalaya](apps/himalaya.md) | [ortie](apps/ortie.md) | [carillon](apps/carillon.md) | [m2m](apps/m2m.md) |
| Plumbing | | | | [sirup](apps/sirup.md) | | |
| Compose | | | [mml](apps/mml.md) | | | |

## Store (read this before Maildir-only designs)

| Card | Status |
|------|--------|
| [store/pimdir.md](store/pimdir.md) | draft-01 spec |
| [store/io-pimdir.md](store/io-pimdir.md) | reference Rust impl |

## Apps (CLIs)

| Card | Ecosystem status |
|------|------------------|
| [apps/neverest.md](apps/neverest.md) | beta |
| [apps/himalaya.md](apps/himalaya.md) | stable (CLI) |
| [apps/ortie.md](apps/ortie.md) | stable |
| [apps/sirup.md](apps/sirup.md) | early |
| [apps/mml.md](apps/mml.md) | stable |
| [apps/carillon.md](apps/carillon.md) | early (watch; Mirador successor) |
| [apps/m2m.md](apps/m2m.md) | in development |

## Libraries (I/O-free Rust; host orchestrates apps, not FFI v1)

| Card | Ecosystem status |
|------|------------------|
| [libs/stream.md](libs/stream.md) | stable |
| [libs/io-http.md](libs/io-http.md) | stable |
| [libs/io-imap.md](libs/io-imap.md) | stable |
| [libs/io-smtp.md](libs/io-smtp.md) | stable |
| [libs/io-oauth.md](libs/io-oauth.md) | stable |
| [libs/io-pim-discovery.md](libs/io-pim-discovery.md) | early |
| [libs/io-jmap.md](libs/io-jmap.md) | early |
| [libs/io-gmail.md](libs/io-gmail.md) | early |
| [libs/io-msgraph.md](libs/io-msgraph.md) | early |
| [libs/io-maildir.md](libs/io-maildir.md) | early |
| [libs/io-m2dir.md](libs/io-m2dir.md) | early |
| [libs/io-managesieve.md](libs/io-managesieve.md) | (Himalaya sieve; stable family) |
| [libs/io-sasl.md](libs/io-sasl.md) | stable |
| [libs/io-starttls.md](libs/io-starttls.md) | stable |

## Clone tiers

See [clone-manifest.yaml](clone-manifest.yaml). Default local clones: **A_must + B_core** (23 repos). Refresh: skill `reference.md`.

## Out of scope for host v1

[frozen/INDEX.md](frozen/INDEX.md), [community/INDEX.md](community/INDEX.md), Himalaya TUI, mobile/GTK shells.

## Related host docs

- [ai-provider-seam.md](../ai-provider-seam.md) (Polypus only)
- [pimalaya-setup.md](../pimalaya-setup.md) (operator onboarding)

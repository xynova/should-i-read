---
name: host-classify-store
description: >-
  Operate the should-i-read classify ledger (host SQLite, object_hash keys,
  mail status / mail similar). Use when classifying mail, inspecting pending vs
  complete, or retrieving similar messages.
---

# Host classify store

**Moral:** Pimdir holds mail bytes. The host SQLite file holds classify state and embeddings. Agents MUST treat those as two stores.

Operator path: [`should-i-read-operator`](../should-i-read-operator/SKILL.md). SQLite driver rules: [`hoststore-sqlite`](../hoststore-sqlite/SKILL.md).

## When to load

- `mail report`, `mail sync` classify, `mail status`, `mail similar`
- Inspecting `tmp/mail-classify-progress.json` vs the live ledger
- Changing `store.path` in operator `config.yaml`

## Constraints

**CONSTRAINT:** The live ledger MUST be `store.path` (default Application Support / XDG `.../should-i-read/host/should-i-read.db`). MUST NOT write classify state into `pimdir.db` or keep a live JSON ledger.

- Enforcement: classify CLI opens `hoststore.CreateStore`; grep for `SaveProgress` in classify path
- Violation: STOP, persist via `internal/hoststore` only

CORRECT:
```text
should-i-read mail report
→ hoststore classify_state + embeddings + attention nouls + clusters (fingerprint resume)
```

Essence skim resume uses `essence_cache` keyed by `object_hash` plus essence/escalate model ids plus essence prompt hash (not classify_state). The host adapter implements taxonomy `harness.EssenceCache`; `PrefetchEssence` on the harness fills attach ops before Operate.

After kind classify, `mail report` scores attention per complete message (Jev `/v1/systemone`), rolls up `clusters[].attention`, and sorts clusters by urgency in human output. Date-window reports set `window_from` / `window_to` on the artifact; per-message rows still key off `object_hash` in `classify_state`. Rerun the same `--from`/`--to` while `omitted` or `attention_omitted` is non-zero when `taxonomy.classify_max` caps a busy week.

**CONSTRAINT:** Cluster TLDR cache MUST use `clusters` and `cluster_members` in the host db. Fingerprint is `v1` + group key + sorted member hashes. `PutCluster` overwrites the live row for each `cluster_id` when a later window changes members; weekly `tmp/unwanted-report-*.json` files are snapshots. MUST NOT store cluster state only in `tmp/unwanted-report-*.json`.

- Enforcement: `PutCluster` / `GetClusterByFingerprint` in `internal/hoststore`
- Violation: STOP, persist cluster rows in hoststore only

PROHIBITED:
```text
Append rows only to tmp/mail-classify-progress.json as the live ledger
```

**CONSTRAINT:** Join key MUST be pimdir `object_hash`. `mail similar` MUST use stored vectors for `polypus.embed_model` (brute-force cosine). Missing vector MUST fail with `not_found` (classify first).

- Enforcement: CLI `mail similar --json` keys `object_hash`, `neighbors[].cosine`
- Violation: STOP, do not call Polypus for similar

**CONSTRAINT:** `tmp/mail-classify-progress.json` is import-only. MUST import once when the store is empty. MUST NOT dual-write after import.

- Enforcement: `ImportProgress` returns 0 when any classify_state rows exist
- Violation: STOP, remove JSON writes from classify

## Checklist (binary)

| Check | Method | Pass | Fail |
|-------|--------|------|------|
| Ledger path | `config show` / `mail status --json` `classify_store.path` | host db file | repo `tmp/` path |
| Status counts | `mail status --json` | `complete` / `error` / `running` / `pending` | missing key |
| Similar | `mail similar --help` | `--k` and `--json` | command missing |
| Pimdir read-only | classify code | no writes to pimdir.db | host tables in pimdir |

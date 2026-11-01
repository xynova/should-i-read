# Report-only unwanted mail evaluation

First milestone after Polypus wiring: classify and cluster unwanted mail, produce TLDRs, **do not mutate the mailbox**.

## Goal

Decide whether taxonomy-backed classification is accurate enough to justify quarantine later. Done looks like a reproducible report: per-message leaf assignments (and optional catalog aliases), then later cluster membership and short cluster summaries, with no deletes, moves, or mark-read side effects.

## Roles

| Component | Owns | Does not own |
|-----------|------|--------------|
| Host mail lane (Neverest + pimdir) | Sync, local storage, post-sync classify, mail export | Direct OpenRouter or TypeSafe calls |
| Polypus | All model HTTP (`/v1/*`), allow-lists, breakers | Mailbox semantics |
| Taxonomy harness (host seats) | Walk: Judge + Author on `inbox-mail`. Attach: Essence + embed + optional Walk reinforce on `inbox-kind` | Mailbox mutation |
| Chat/summary model (via Polypus) | Cluster TLDR text (`polypus.tldr_model` or `classify_model`) | Per-message leaf assignment |

## Pipeline (report only)

```mermaid
flowchart TD
  sync[mail sync]
  classify[Taxonomy classify via Polypus chat]
  report[tmp/unwanted-report-*.json]
  export[mail export optional]
  cluster[Cluster by term or domain]
  tldr[TLDR via Polypus chat]
  sync --> classify
  classify --> cluster
  cluster --> tldr
  tldr --> report
  export --> classify
```

1. Sync mail with `go tool task mail:sync`. After a successful fetch, the host classifies newly fetched unique messages (cap per `taxonomy.classify_max`; use `--no-classify` to skip). Polypus down after sync: replica stays; classify skipped; exit 0.
2. Resume or backfill with `go tool task mail:report` / `should-i-read mail report` (lists recent pimdir rows, reads `--in` export JSON, or selects a UTC date window). Requires Polypus up (fail closed) and a SystemOne judge resolved via `polypus check --classify` or successful in-process discovery.
   - **Date window:** `--from` and `--to` as `YYYY-MM-DD` (UTC midnight), half-open `[from, to)`. Filters live pimdir rows by `items.sort_key` within `taxonomy.collections`. No row cap unless you set `--limit`. Cannot combine with `--in`.
   - **Hop cap:** `taxonomy.classify_max` limits new Operate and attention calls per run. Rerun the same `--from`/`--to` while `omitted` or `attention_omitted` is non-zero.
   - **Example (September 2026, one week each):** `go tool task mail:report -- --from 2026-09-01 --to 2026-09-08`, then `2026-09-08`/`2026-09-15`, `2026-09-15`/`2026-09-22`, `2026-09-22`/`2026-10-01`. Day `2026-09-08` belongs only to the second window.
   - **Artifacts:** each run writes `tmp/unwanted-report-<ts>.json` with optional `window_from` / `window_to`. Per-message ledger rows accumulate by `object_hash`. The live cluster cache is overwritten for each `cluster_id` on the latest processed window (weekly JSON files are the snapshots).
3. For each message, taxonomy `Operate` runs **attach** (`taxonomy.strategy: attach`; legacy `walk` is treated as attach). Attach: Essence is three Polypus chat hops (WHY slave, TAGS slave, KIND slave), then embeddings, cosine alias or Walk reinforce or breadcrumb create on `inbox-kind`. Repeating CI subjects may skip Essence: after `taxonomy.shape_min_repeats` (default 3) hits of the same sender domain and templated subject under the current Essence prompt hash, the host promotes a row in `mail_shapes` and later mail with that fingerprint gets a Jev yes/no (`taxonomy.shape_confirm_min`, default 0.90). A match Completes with `strategy=shape-confirm` and copies KIND; a miss falls through to Essence. Default allow-listed last slots are `pipeline-failure`, `workflow-failure`, `pull-request`, and `release`. `mail bench` forces `shape_confirm` off. Report rows include `path`, and attach rows add `kind`, `about`, `shape`, `cosine` (leaf attach match), `description_cosine` (email vs ABOUT when `taxonomy.description_cosine_mode` is `report` or `repair`), `canonical_term_id`, `reinforced`. Breadcrumb ABOUT/SHAPE must pass host layout/length gates before catalog apply. Accepted drafts update the active catalog YAML unless `--no-apply`.
4. Cluster messages in the report by `canonical_term_id` / `term_id`, then sender domain; optional embedding attach via `cluster.min_cosine`. One TLDR per cluster via Polypus chat (cached in host SQLite by fingerprint).
5. Write `tmp/unwanted-report-<timestamp>.json` with `cluster_id` on each message and `clusters[]` summaries. This-run counters `shape_confirmed` and `shapes_promoted` appear in JSON and, when greater than zero, as human lines under `Classified N`. Resume-complete rows keep `strategy` from the ledger but do not increment those this-run counters. **No mailbox trash/spam/delete API calls.**

## Output schema (minimum)

```json
{
  "mode": "report_only",
  "polypus_base_url": "http://127.0.0.1:1320",
  "messages": [
    {
      "id": "…",
      "cluster_id": "c1",
      "decisions": {
        "is_unwanted": 0.97,
        "category": "newsletter",
        "urgency": 0.1
      }
    }
  ],
  "clusters": [
    {
      "id": "c1",
      "size": 12,
      "tldr": "Weekly retail promos from the same ESP.",
      "suggested_action": "quarantine_candidate"
    }
  ]
}
```

## Thresholds (draft; tune after first run)

| Gate | Draft rule | Effect |
|------|------------|--------|
| Auto-flag for review | `is_unwanted >= 0.90` and `category ∈ {spam, phishing, newsletter, promo}` | Appears in report only |
| Hold for human | `0.70 <= is_unwanted < 0.90` | Separate "uncertain" section |
| Leave alone | `is_unwanted < 0.70` or `category = legitimate` | Omitted from unwanted clusters |
| Quarantine automation | Not allowed in this milestone | Requires explicit later approval |
| Permanent delete | Never in v1 | Out of scope |

## Explicit non-goals

- Moving messages to Spam or Trash
- Permanent deletion
- Marking messages read
- Training on full mailbox without a sampled window
- Calling TypeSafe / Jev without Polypus

## Exit criteria to unlock quarantine later

- Evaluation set of at least 100 labeled messages (human-checked)
- Precision on `is_unwanted` for the auto-flag band meets an agreed target (set after first report)
- Zero false-positive phishing misses on the eval set for the auto-flag band, or documented accepted risk
- Operator can reproduce the report from CLI with Polypus up and mail synced

## Shipped slice (taxonomy v0.3.0+)

- Post-sync classify on fetch hunks; `mail report` for resume/backfill.
- INBOX (or `taxonomy.collections`) item pick, then taxonomy `Operate` per message (attach on `inbox-kind`; repeating allow-listed CI templates may Jev-confirm and skip Essence).
- `taxonomy.strategy`: `attach` (default; legacy `walk` is normalized to attach). Requires `polypus.embed_model` and Essence chat (`polypus.essence_model` or classify author); Operate timeout is `operateTimeoutAttach` (5 minutes). Thresholds: `attach_min_cosine` (0.80), `walk_reinforce_min` (0.70).
- `taxonomy.classify_max` caps Operate (Polypus) calls per run.
- `taxonomy.author_on_skip: false` (default) skips Author chat on Judge skip; set `true` to restore draft proposals. When Author runs, the host cleans the latest body (reply parser + line denoise + extractive cap) before Granite sees it.
- Strop `JobRunner` generator `mail_classify` wraps taxonomy `Operate`; Polypus HTTP only inside Judge/Author/Essence/Embed seats.
- Artifacts: `tmp/unwanted-report-*.json` (snapshot) with `clusters[]`, per-message `cluster_id`, `strategy` (`attach` or `shape-confirm`), `suggested_action` (`review` or `quarantine_candidate` when `cluster.quarantine_terms` matches), nested `attention` on messages and clusters (`urgency`, `needs_reply`, `involves_decision` from Jev SystemOne via Polypus; report-only), and this-run `shape_confirmed` / `shapes_promoted`. Human classify box prints `shape-confirmed N (Jev, skipped Essence)` and `shapes promoted M` when those counts are greater than zero. When Essence parse fails after retries, the last chat completion is written to `tmp/essence-parse/<object_hash>.json` (`essence_dump_dir` on the report). Live classify state, embeddings, attention nouls, cluster TLDR cache, and repeating-template registry: host SQLite (`store.path`; tables `classify_state`, `embeddings`, `clusters`, `cluster_members`, `mail_shapes`, `essence_cache`). `tmp/mail-classify-progress.json` is import-only when the ledger is empty.
- Config: `cluster.min_cosine`, `cluster.quarantine_terms`, `cluster.attention_true_min` (default 0.50), optional `polypus.tldr_model`. Shape-confirm knobs under `taxonomy:`: `shape_confirm` (default true), `shape_min_repeats` (3), `shape_confirm_min` (0.90), `shape_allowed_leaves`. Promote looks up `essence_cache` with the same joined WHY/TAGS/KIND prompt hash the live cache uses, not the tag-less default prompt.

### Live shape-confirm check (2026-11-01)

Reopened repeating GitHub/GitLab templates already in the operator ledger (not a prospective unlabeled eval set; mailbox unchanged). Fingerprint fix required: promote GetEssence must use the joined Essence prompt hash. The third batch used a temp `--config` with `judge_model: cf_local/typesafe/jev`; live operator config still pins `openrouter/perplexity/pplx-decider-v1.2`.

| Run | Messages | Judge | `shapes_promoted` | `shape_confirmed` | Attach leftover | Artifact |
|-----|----------|-------|-------------------|-------------------|-----------------|----------|
| First batch | 32 | pplx-decider | 4 | 18 | 14 (12 seed hits, 2 Fabric Jev misses that landed `version`) | `tmp/shape-batch-report.json` |
| Second batch | 100 | pplx-decider | 11 | 67 | 33 (exactly 11 new templates × 3 seed hits; zero Jev misses) | `tmp/shape-batch-100-report.json` |
| Third batch | 100 | local Jev | 0 | 25 | 75 (56 majordomo Jev mismatches, 17 PR comment pairs, 2 PR-run-failed copies) | `tmp/shape-batch-200-report.json` |
| Combined | 232 | mixed | 15 | 110 | 122 | three artifacts above |

Hop savings vs a full-attach baseline (three Essence chats plus one embed per message). A `shape-confirm` match spends one Jev call and skips Essence. A Jev miss spends Jev and still runs Essence.

| Run | Essence chats saved | Essence chats spent | Embeds saved | Jev paid (match + miss) |
|-----|---------------------|---------------------|--------------|-------------------------|
| First batch | 54 | 42 | 18 | 20 (18 + 2) |
| Second batch | 201 | 99 | 67 | 67 (67 + 0) |
| Third batch | 75 | 225 | 25 | 81 (25 + 56) |
| Combined | **330** of 696 | 366 | **110** of 232 | 168 (110 + 58) |

After the first two runs the live `mail_shapes` table had **15 active** templates and **85** this-run `shape-confirm` Completes. The 17 leftover Fabric release mails still stored as `pull-request` from October were Jev-matched to the active `release` shape.

After the third run (local Jev), combined this-run skips are **110**. `mail_shapes` is **14 active**, **10 accumulating** (PR pairs at 2 hits, below `shape_min_repeats`), and **1 disabled**: `[xynova/majordomo-tower] run failed: majordomo-context-digest - main ({sha})` (`confirm_count` 5 from pplx-decider, `reject_count` 56 from local Jev). Local Jev is stricter than pplx-decider on that same active shape. Operator mailbox unchanged.

## Essence benchmark (frozen input)

Operator classify uses the **checklist** repair contract by default (high-effort Essence escalation plus the silent validation checklist on ABOUT rewind). Bench `baseline` and `thinking` only exist as ablations (`SHOULD_I_READ_BENCH_*`); iterate prompt copy against **checklist**, then compare ablations if needed.

Compare Essence prompt variants on the **same** 100 messages (live Polypus, report-only, fresh ledger per chunk):

1. `go tool task mail:export -- --limit 500 --out bench/input-export.json` (needs at least 100 unique `object_hash` values).
2. `./bin/should-i-read mail bench freeze --in bench/input-export.json` writes `bench/input-100.json`, `bench/chunks/c1.json`…`c5.json`, and `bench/input-rev.txt`. Use `--out-dir bench/eval-100` for a dated freeze that does not overwrite an older dump.
3. Edit `bench/labels-20.json` (`expected_leaf`, `required_terms`) after `mail bench labels-stub` if needed. Follow **Last-slot gold** below. Committed smoke labels live in `testdata/bench/labels.json` (revision in `input-rev.txt`).
4. `go tool task polypus-check -- --classify`, then smoke the production-like variant on committed fixtures (no live pimdir): `./bin/should-i-read mail bench --fresh --variant checklist --chunks testdata/bench/chunks --labels testdata/bench/labels.json --input-rev testdata/bench/input-rev.txt`.
5. Full matrix (sequential, live freeze under gitignored `bench/`): run each variant with `--fresh` so the classify ledger under `tmp/bench/<variant>/` is wiped first (otherwise bench reuses cached classifies and wall time is misleading). Example: `./bin/should-i-read mail bench --fresh --variant baseline --chunks bench/chunks` → `tmp/bench/<variant>-artifact.json` and `tmp/bench/<variant>-score.json`. Or `go tool task mail:bench-matrix` for baseline, thinking, and checklist on `testdata/bench` fixtures. Shape-confirm is forced off for `mail bench` (`SourceBench`); those scores are Essence-only.

### Last-slot gold

`expected_leaf` is the **last segment of Essence `KIND`** (`family > class > leaf`). It is the catalog leaf, not a free-form category and not the parent stuffed into the leaf. Bench `leaf_accuracy` compares only that last segment.

KIND is a three-level tree. Put the distinctive word in **one** slot. If class (or family) is already `security`, `meeting`, `event`, or `ci`, the leaf stays the short event type (`alert`, `reminder`, `webinar`, `pull-request`). Hyphenating the parent into the leaf (`security-alert`, `meeting-reminder`, `webinar-invitation`, `pull-request-comment`) fails last-slot score against a legal tree, and `security > … > security-alert` is rejected as a parent-prefix repeat.

| Mail | KIND last slot (gold) | Do not use as gold |
|------|----------------------|--------------------|
| LinkedIn “just messaged you” digest | `message` | `new-message` |
| Google or broker security notice | `alert` under a `security` parent | `security-alert` |
| Calendar reminder | `reminder` | `meeting-reminder` |
| Webinar invite | `webinar` | `webinar-invitation` |
| GitHub Actions run failed | `workflow-failure` | `failure`, `ci-failure` |
| Bot comment on a pull request | `pull-request` | `pull-request-comment`, `workflow-failure` (even if the comment reports a quality gate) |
| Published newsletter post | `article` | `personal-communication` as the leaf |
| Confirm tax residency / similar account confirm | `confirmation-request` | `tax-residency-confirmation` |
| Sub-processor or support tooling change | `update` | `processor-change` |

Do not copy a unique ABOUT detail into `expected_leaf`. Distinguishing phishing from a Google sign-in lives in the **parent path**, not in two different last-slot names, unless the events are truly different reusable types (for example `workflow-failure` vs `pipeline-failure`).

Bench-only env (unset in normal classify): `SHOULD_I_READ_BENCH_REASONING` (`high`|`off`), `SHOULD_I_READ_BENCH_FEEDBACK` (`checklist`|`short`). Variants: baseline (`off`+`short`), thinking (`high`+`short`), checklist (`high`+`checklist`). Each chunk probes Essence (`polypus.essence_model`) and does not smoke Author (`polypus.classify_model`) or embeddings (`polypus.embed_model`). Operator classify still fail-closes on those probes. Gateway is `polypus.base_url` / `POLYPUS_BASE_URL` (homelab: `https://polypus.manticore-powan.ts.net`).

## Next implementation slice

1. Tune eval thresholds and `is_unwanted` scoring against labeled samples (aspirational `decisions` block in the schema below).
2. Reversible quarantine automation only after exit criteria pass (still no permanent delete in v1).

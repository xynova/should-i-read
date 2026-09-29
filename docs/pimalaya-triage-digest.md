# Triage signature and daily digest (report-only)

Future strop JobRunner loop classifies MIME excerpts via Polypus. This slice defines the contract only; no mailbox mutation.

## Strop signature fields

| Field | Type | Notes |
|-------|------|-------|
| `ShouldRead` | bool | Operator wants to read soon |
| `Category` | enum | `actionable`, `personal`, `newsletter`, `notification`, `cold_outreach`, `spam`, `other` |
| `UrgencyScore` | int | 0 (low) to 10 (immediate) |
| `OneLineRationale` | string | Single sentence for digest |

## Digest artifact

Written under `tmp/` as JSON (and optional markdown later). Mode is always `report_only`.

```json
{
  "mode": "report_only",
  "polypus_base_url": "http://127.0.0.1:1320",
  "generated_at": "2026-01-01T12:00:00Z",
  "must_read": [],
  "fyi": [],
  "unwanted_clusters": []
}
```

Each bucket entry references a pimdir `seq`, subject, sender, triage fields, and optional excerpt.

## CLI scaffold

`should-i-read pim digest-check` runs Polypus health and prints the empty digest shape (no classification yet).

---
name: hoststore-sqlite
description: >-
  Change should-i-read internal/hoststore: modernc SQLite, numbered golang-migrate
  files, write actor, no CGO. Use when editing migrations, DSN, Claim/Complete, or embeddings.
---

# Hoststore SQLite

**Moral:** Embedded SQLite is pure Go. Writes are sequential. Schema changes are numbered migrations applied once at Open.

golang-quality **C24** (golang-migrate) and **C26** (modernc). Product usage: [`host-classify-store`](../host-classify-store/SKILL.md).

## When to load

- Editing `internal/hoststore/`
- Adding a migration
- Opening or pooling the classify ledger

## Constraints

**CONSTRAINT:** MUST use `modernc.org/sqlite` and `sql.Open("sqlite", dsn)` with `_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(30000)`. MUST `SetMaxOpenConns(1)` on the writer pool. NEVER add `github.com/mattn/go-sqlite3`, driver name `sqlite3`, or `CGO_ENABLED=1` for this store.

- Enforcement: `CGO_ENABLED=0 go test ./internal/hoststore/...`; grep `mattn` / `sqlite3`
- Violation: STOP, switch to modernc DSN, re-test with CGO off

CORRECT:
```go
import _ "modernc.org/sqlite"
sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(30000)")
```

PROHIBITED:
```go
import _ "github.com/mattn/go-sqlite3"
sql.Open("sqlite3", path+"?_foreign_keys=on")
```

**CONSTRAINT:** Schema MUST live in `internal/hoststore/migrations/NNNNNN_*.up.sql` (+ `.down.sql`). Open MUST apply pending versions once via golang-migrate (`embed` + `source/iofs` + `database/sqlite.WithInstance` on the existing `*sql.DB`). MUST NOT call `migrate.Close()` on that instance (it would close the shared DB). MUST NOT run full DDL on every insert.

- Enforcement: Stage 2 tests Open twice; `schema_migrations` owned by golang-migrate
- Violation: STOP, restore golang-migrate bootstrap

**CONSTRAINT:** Classify writes MUST go through Claim then Complete or Fail on one sequential owner (`Runner.Classify`). Attention scores use `PutAttention` on the same runner goroutine (no second Claim). MUST NOT upsert from extra goroutines. Injected `Now` stamps `updated_at` (C25). Store methods MUST require a context deadline (C8).

- Enforcement: grep `time.Now` in hoststore write paths; nil ctx tests
- Violation: STOP, inject clock / fail closed

## Checklist (binary)

| Check | Method | Pass | Fail |
|-------|--------|------|------|
| Driver | `go.mod` + blank import | modernc only | mattn present |
| CGO | `CGO_ENABLED=0 go test ./internal/hoststore/...` | exit 0 | compile/link CGO |
| Migrate | Open twice | no error / ErrNoChange | custom ApplyPending loop |
| Writes | Claim/Complete/Fail | sequential runner | concurrent Store writers |

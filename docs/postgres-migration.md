# PostgreSQL Migration Guide

This document outlines the steps to migrate from SQLite to PostgreSQL when scaling beyond a single node.

## When to Migrate

- Multiple application instances need shared database access
- Dataset exceeds ~1 GB (SQLite WAL performance degrades)
- Need concurrent write throughput beyond what SQLite provides
- Require replication, point-in-time recovery, or streaming backups

## Schema Changes

SQLite and PostgreSQL SQL differ in several areas:

| SQLite | PostgreSQL |
|--------|------------|
| `INTEGER PRIMARY KEY AUTOINCREMENT` | `BIGSERIAL PRIMARY KEY` |
| `DATETIME` | `TIMESTAMPTZ` |
| `TEXT NOT NULL DEFAULT 'active'` | Same (works as-is) |
| `PRAGMA journal_mode = WAL` | Not needed (WAL is default) |
| `PRAGMA foreign_keys = ON` | Foreign keys are always enforced |
| `VACUUM INTO ?` | `pg_dump` or `pg_basebackup` |
| `GROUP_CONCAT(col, '||')` | `STRING_AGG(col, '||')` |
| `CURRENT_TIMESTAMP` | `NOW()` or `CURRENT_TIMESTAMP` |

## Migration Steps

1. **Add a PostgreSQL driver** — Replace `modernc.org/sqlite` with `github.com/jackc/pgx/v5/stdlib`:
   ```bash
   go get github.com/jackc/pgx/v5
   ```

2. **Switch `sql.Open`** — Change driver name from `"sqlite"` to `"pgx"` and use a connection string:
   ```go
   db, err := sql.Open("pgx", os.Getenv("DATABASE_URL"))
   ```

3. **Update migrations** — Replace SQLite-specific syntax:
   - `AUTOINCREMENT` to `BIGSERIAL`
   - `DATETIME` to `TIMESTAMPTZ`
   - Remove all `PRAGMA` statements
   - Replace `GROUP_CONCAT` with `STRING_AGG`
   - Replace `VACUUM INTO` backup with `pg_dump`

4. **Update `ensureColumn`** — Use `information_schema.columns` instead of `PRAGMA table_info`:
   ```sql
   SELECT 1 FROM information_schema.columns
   WHERE table_name = $1 AND column_name = $2
   ```

5. **Replace parameterized placeholders** — SQLite uses `?`, PostgreSQL uses `$1, $2, ...`. The `pgx` driver handles this automatically when using `database/sql`.

6. **Export data from SQLite**:
   ```bash
   sqlite3 data/app.db ".dump" > dump.sql
   ```

7. **Import into PostgreSQL** — Adjust the dump SQL for PostgreSQL syntax and import, or use a tool like `pgloader`.

## Environment Variable

Replace `DB_PATH` with `DATABASE_URL`:
```
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=require
```

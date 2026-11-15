# Kratos Code Review — SQL Rules

> Extends `default.md`. These rules apply to any file containing SQL (`.sql`), including migrations, DDL, DML scripts, and stored procedures.
> Project-specific overrides live in `.claude/.Arena/review-rules/sql.md`.

Based on common migration-safety and query-safety practices for Postgres/MySQL-family databases. Dialect-specific notes are called out where behavior diverges.

---

## Destructive Statement Safety (Tier 1–2)

These are correctness and data-loss issues. Treat as `[BLOCKER]`.

### unscoped-update-delete — `[BLOCKER]`

An `UPDATE` or `DELETE` statement must have a `WHERE` clause, unless truncation of the entire table is the explicit, stated intent.

```sql
-- BLOCKER — wipes every row
UPDATE accounts SET status = 'inactive';
DELETE FROM sessions;

-- Correct
UPDATE accounts SET status = 'inactive' WHERE last_login < '2024-01-01';
DELETE FROM sessions WHERE expires_at < now();
```

### destructive-statement-ordering — `[BLOCKER]`

A `DROP TABLE`/`DROP COLUMN` must not execute before the statements that migrate or archive its dependent data.

```sql
-- BLOCKER — data below the DROP never runs
DROP TABLE legacy_orders;
INSERT INTO orders SELECT * FROM legacy_orders;

-- Correct — migrate first, drop after data is confirmed moved
INSERT INTO orders SELECT * FROM legacy_orders;
DROP TABLE legacy_orders;
```

### unparameterized-dynamic-sql — `[BLOCKER]`

Dynamic SQL built inside a stored procedure or script (`EXEC(@sql)`, `format('... %s ...', input)`, string concatenation) must bind values as parameters, never interpolate them into the SQL text.

```sql
-- BLOCKER — injectable
EXEC('SELECT * FROM users WHERE id = ' + @user_id);

-- Correct
EXECUTE format('SELECT * FROM users WHERE id = $1') USING user_id;
```

---

## Migration Reversibility & Deploy Safety (Tier 2 / Tier 6)

### non-reversible-migration — `[WARNING]`

A migration that alters schema or data must have a corresponding down/rollback path (a paired `down.sql`, a documented reverse migration, or an explicit "irreversible by design" note with reasoning).

```sql
-- WARNING — no way back if this ships broken
-- up.sql
ALTER TABLE users DROP COLUMN legacy_flag;

-- Correct — down.sql exists
-- down.sql
ALTER TABLE users ADD COLUMN legacy_flag boolean DEFAULT false;
```

### blocking-ddl-lock-escalation — `[BLOCKER]`

DDL that rewrites or locks a large/hot table (`ALTER TABLE ... ADD COLUMN` with a non-null default on old Postgres/MySQL versions, `CREATE INDEX` without `CONCURRENTLY` on Postgres, full-table `ALTER TABLE` on MySQL without an online-schema-change tool) must use the non-blocking form the dialect supports, or be flagged for a maintenance window.

```sql
-- BLOCKER — takes an ACCESS EXCLUSIVE lock for the full build on Postgres
CREATE INDEX idx_orders_user_id ON orders (user_id);

-- Correct
CREATE INDEX CONCURRENTLY idx_orders_user_id ON orders (user_id);
```

### not-null-without-default — `[BLOCKER]`

Adding a `NOT NULL` column to a populated table must supply a `DEFAULT`, or backfill the column before the constraint is added — otherwise the statement fails against existing rows.

```sql
-- BLOCKER — fails immediately on any existing row
ALTER TABLE users ADD COLUMN tier text NOT NULL;

-- Correct
ALTER TABLE users ADD COLUMN tier text NOT NULL DEFAULT 'free';
```

### drop-without-two-phase-deploy — `[BLOCKER]`

A migration that drops a column or table must not ship in the same deploy as the code change that stops writing to it — old running instances (or read replicas) will still reference it mid-rollout. Split into a "stop using" deploy and a later "drop" deploy.

```sql
-- BLOCKER — if application code in the same release still selects/writes
-- this column, in-flight old-code instances break during rollout
ALTER TABLE users DROP COLUMN legacy_email;

-- Correct — ship this only in a follow-up migration, after the
-- code that reads/writes legacy_email has been fully rolled out
```

### unwrapped-multi-statement-migration — `[WARNING]`

A migration with multiple dependent DML/DDL statements should be wrapped in a transaction (where the dialect supports transactional DDL) so a mid-script failure doesn't leave schema/data half-migrated.

```sql
-- WARNING — a failure on the second statement leaves the migration half-applied
ALTER TABLE orders ADD COLUMN total_cents integer;
UPDATE orders SET total_cents = round(total * 100);

-- Correct
BEGIN;
ALTER TABLE orders ADD COLUMN total_cents integer;
UPDATE orders SET total_cents = round(total * 100);
COMMIT;
```

---

## Indexing & Query Hygiene (Tier 7)

### missing-fk-index — `[WARNING]`

A foreign-key column should have a supporting index — without one, joins on the FK and cascading deletes/updates commonly force full-table scans.

```sql
-- WARNING — no index backing the FK
ALTER TABLE orders ADD COLUMN user_id integer REFERENCES users(id);

-- Correct
ALTER TABLE orders ADD COLUMN user_id integer REFERENCES users(id);
CREATE INDEX CONCURRENTLY idx_orders_user_id ON orders (user_id);
```

### unjustified-cascade — `[WARNING]`

`ON DELETE CASCADE` / `ON UPDATE CASCADE` on a foreign key should be an explicit, stated decision, not the default — an unreviewed cascade silently deletes/updates dependent rows the author may not have intended.

```sql
-- WARNING — silently deletes every order when a user is deleted
user_id integer REFERENCES users(id) ON DELETE CASCADE
```

---

## Maintainability (Tier 8)

### select-star-in-shared-object — `[WARNING]`

A view or stored procedure that other code depends on must not use `SELECT *` — adding or reordering a column silently changes the contract for every downstream consumer.

```sql
-- WARNING — downstream consumers break/silently change behavior
-- when the base table's columns change
CREATE VIEW active_orders AS
  SELECT * FROM orders WHERE status = 'active';

-- Correct — explicit column list is the stable contract
CREATE VIEW active_orders AS
  SELECT id, user_id, total_cents, status FROM orders WHERE status = 'active';
```

---

## What Does NOT Count as a Violation

Do not flag:
- `SELECT *` in ad-hoc query files or one-off scripts not depended on by other code
- Choosing surrogate (`id serial`) vs natural primary keys (schema design preference)
- Denormalization for read performance (project-specific trade-off)
- Naming convention choices (`snake_case` vs `camelCase` columns) unless the project has a documented convention

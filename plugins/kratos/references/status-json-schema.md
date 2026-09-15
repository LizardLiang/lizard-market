# status.json Schema Reference

The Kratos CLI owns `.claude/feature/<name>/status.json`. `kratos pipeline init` creates the file. `pipeline update`, `pipeline set-pending`, `pipeline tasks complete` and `kratos check --verify` mutate it. This file documents exactly the fields those commands write. Agents edit the file by hand only when the binary is unavailable, with timestamps from `kratos now`. `pipeline get --compact` omits `history[]` and `check_failures[]`.

## Top-level fields

| Field | Written by | Value |
|-------|-----------|-------|
| `feature` | init | Feature name; equals the directory name |
| `description` | init | `--description` |
| `priority` | init | `--priority`; default `P2` |
| `stage` | init, update | Current stage key. Init writes `1-prd`. Update rewrites it when `--status` is `in-progress` or `complete`. |
| `created` | init | RFC3339 timestamp |
| `updated` | every write | RFC3339 timestamp |
| `pending_stage` | set-pending | Stage that `kratos check --init` expects next. `--stage ""` deletes the key. |
| `documents` | update `--document` | Map of stage key to document path. Init writes `{}`. |
| `history[]` | update, tasks complete | Append-only. Entry: `{timestamp, stage, action, verdict?}`. `action` is `status changed from '<old>' to '<new>'` or `tasks completed: <ids>`. |
| `pipeline` | init | One object per stage key `1-prd` … `9-review` |

## Per-stage fields

| Field | Written by | Value |
|-------|-----------|-------|
| `status` | init, update | `in-progress`, `complete`, `blocked`, `ready`, `skipped`. Init sets stage 1 `in-progress`, stage 3 `skipped`, all others `blocked`. Update of a missing stage key creates it with `pending` first. |
| `assignee` | init | athena, nemesis, daedalus, hephaestus, apollo, artemis, ares, hera, hermes |
| `document` | init, update `--document` | Deliverable file name |
| `gate` | init | `{requires: [stage keys], condition: "<text>"}`. Absent on `1-prd`. |
| `optional` | init | `true` on `3-decomposition` only |
| `started` | update | Set when status becomes `in-progress`. Backfilled on `complete` when still null. |
| `completed` | update | Set on every `complete` |
| `summary` | update `--summary` | 2–3 sentences for downstream agents. Any stage. |
| `mode` | init (null), update `--mode` | `ares` or `user`. `7-implementation` only. |
| `tasks` | init (null), tasks complete | `{total, completed, items[]}`. Item: `{id, name, file?, status, completed_at?}`. `7-implementation` only. |
| `check_failures[]` | check --verify | Append-only. Entry: `{timestamp, tier, checks_failed[], retries_exhausted: true}`. Written when the retry limit is exhausted. |
| verdict fields | update `--verdict` | See below |

## Verdicts

`--verdict` is accepted on stages 2, 5, 8 and 9 only. The CLI lower-cases the value and normalizes `changes-requested` and `changes required` to `changes-required`. The `history[]` entry stores the verdict as typed.

| Stage | Agent | Field | Values |
|-------|-------|-------|--------|
| `2-prd-review` | Nemesis | `nemesis_verdict`, mirrored to `verdict` | approved, revisions, rejected |
| `5-spec-review-sa` | Apollo | `verdict` | sound, concerns, unsound |
| `8-prd-alignment` | Hera | `alignment_verdict` | aligned, gaps, misaligned |
| `9-review` | Hermes | `code_review_verdict` | approved, changes-required |
| `9-review` | Cassandra | `risk_verdict` | clear, caution, blocked |

Stage 9 vocabularies are disjoint, so Hermes and Cassandra never overwrite each other. Verdict thresholds live in each agent definition.

## Stage 7 User Mode

`pipeline tasks complete <id>... | --all` sets `completed_at` on each item. When the last item completes, the same write sets stage 7 `complete` and stage 8 `ready` (`--no-advance` disables this).

## Conflict detection

`kratos pipeline status` reports `health: conflict` when `prd.md` has a newer file modification time than `tech-spec.md`. No CLI command writes a PRD version into the spec stage, so compare `history[]` timestamps when you need the order of events.

## Example

Produced by `pipeline init`, then `update --stage 1 --status complete --document prd.md --summary ...`, `update --stage 2 --status complete --verdict approved --document prd-challenge.md`, `set-pending --stage 4-tech-spec`, and `update --stage 7 --status in-progress --mode ares`. Stages 3–6, 8 and 9 are elided; they keep the init shape (`assignee`, `document`, `gate`, null timestamps, `status: blocked` or `skipped`).

```json
{
  "created": "2026-09-13T16:19:31+08:00",
  "description": "Demo feature",
  "documents": { "1-prd": "prd.md", "2-prd-review": "prd-challenge.md" },
  "feature": "demo",
  "history": [
    { "action": "status changed from 'in-progress' to 'complete'", "stage": "1-prd", "timestamp": "2026-09-13T16:19:31+08:00" },
    { "action": "status changed from 'blocked' to 'complete'", "stage": "2-prd-review", "timestamp": "2026-09-13T16:19:31+08:00", "verdict": "approved" },
    { "action": "status changed from 'blocked' to 'in-progress'", "stage": "7-implementation", "timestamp": "2026-09-13T16:19:31+08:00" }
  ],
  "pending_stage": "4-tech-spec",
  "pipeline": {
    "1-prd": { "assignee": "athena", "completed": "2026-09-13T16:19:31+08:00", "document": "prd.md", "started": "2026-09-13T16:19:31+08:00", "status": "complete", "summary": "PRD with 4 requirements." },
    "2-prd-review": { "assignee": "nemesis", "completed": "2026-09-13T16:19:31+08:00", "document": "prd-challenge.md", "gate": { "condition": "prd.status === 'complete'", "requires": ["1-prd"] }, "nemesis_verdict": "approved", "started": "2026-09-13T16:19:31+08:00", "status": "complete", "verdict": "approved" },
    "7-implementation": { "assignee": "ares", "completed": null, "document": "implementation-notes.md", "gate": { "condition": "test-plan exists", "requires": ["6-test-plan"] }, "mode": "ares", "started": "2026-09-13T16:19:31+08:00", "status": "in-progress", "tasks": null }
  },
  "priority": "P1",
  "stage": "7-implementation",
  "updated": "2026-09-13T16:19:31+08:00"
}
```

*See `<KRATOS_ROOT>/references/agent-protocol.md` for document creation procedures.*

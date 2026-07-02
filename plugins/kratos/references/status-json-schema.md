# status.json Schema Reference

Single source of truth for the feature status file at `.claude/feature/<name>/status.json`.

All agents that read or update status.json MUST follow this schema.

---

## Complete Schema

```json
{
  "feature": "<feature-name>",
  "created": "<ISO8601 timestamp>",
  "updated": "<ISO8601 timestamp>",
  "stage": "<stage-id>",
  "pipeline_status": "in-progress | complete | blocked | abandoned",
  "mode": "normal | eco | power",
  "implementation_mode": "ares | user | null",

    "pipeline": {
    "1-prd": {
      "status": "pending | in-progress | complete",
      "agent": "athena",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["prd.md"],
      "gap_analysis_rounds": 0
    },
    "2-prd-review": {
      "status": "pending | in-progress | complete",
      "agents": ["nemesis"],
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["prd-challenge.md"],
      "nemesis_verdict": "approved | revisions | rejected",
      "verdict": "approved | revisions | rejected"
    },
    "3-decomposition": {
      "status": "skipped | in-progress | complete",
      "agent": "daedalus",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["decomposition.md"],
      "output_targets": ["local", "notion", "linear"]
    },
    "4-tech-spec": {
      "status": "pending | in-progress | complete",
      "agent": "hephaestus",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["tech-spec.md"],
      "based_on_prd_version": "<ISO8601 of prd.md last modified>",
      "summary": "<2-3 sentence digest: key architectural decisions, file count, major components>"
    },
    "5-spec-review-sa": {
      "status": "pending | in-progress | complete",
      "agent": "apollo",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["spec-review-sa.md"],
      "verdict": "sound | concerns | unsound"
    },
    "6-test-plan": {
      "status": "pending | in-progress | complete",
      "agent": "artemis",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["test-plan.md"],
      "summary": "<2-3 sentence digest: total test cases, P0 coverage, key risk areas targeted>"
    },
    "7-implementation": {
      "status": "pending | in-progress | complete | waiting-user",
      "agent": "ares",
      "mode": "ares | user",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["implementation-notes.md"],
      "summary": "<2-3 sentence digest: files created/modified, tests written, any deviations from spec>",
      "tasks": [
        {
          "id": "01",
          "file": "tasks/01-task-name.md",
          "title": "Task title",
          "status": "pending | in-progress | complete",
          "completed_at": "<ISO8601> | null"
        }
      ]
    },
    "8-prd-alignment": {
      "status": "pending | in-progress | complete",
      "agent": "hera",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["prd-alignment.md"],
      "alignment_verdict": "aligned | gaps | misaligned",
      "criteria_total": 0,
      "criteria_verified": 0,
      "coverage_pct": 0
    },
    "9-review": {
      "status": "pending | in-progress | complete",
      "agents": ["hermes", "cassandra"],
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["code-review.md", "risk-analysis.md"],
      "code_review_verdict": "approved | changes-required",
      "risk_verdict": "clear | caution | blocked"
    }
  },

  "history": [
    {
      "timestamp": "<ISO8601>",
      "stage": "<stage-id>",
      "action": "started | completed | revision-requested | skipped",
      "agent": "<agent-name>",
      "notes": "<optional details>"
    }
  ]
}
```

## Field Reference

### Top-Level Fields

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `feature` | string | yes | Feature name (matches directory name) |
| `created` | ISO8601 | yes | When pipeline was initialized |
| `updated` | ISO8601 | yes | Last modification timestamp |
| `stage` | string | yes | Current active stage ID (e.g., "4-tech-spec") |
| `pipeline_status` | enum | yes | Overall pipeline status |
| `mode` | enum | yes | Execution mode (affects model assignments) |
| `implementation_mode` | enum | no | Set at stage 7; "ares" (AI implements) or "user" (task files created) |

### Stage Status Values

| Status | Meaning |
|--------|---------|
| `pending` | Not yet started, waiting for prerequisites |
| `skipped` | Intentionally bypassed (research, decomposition, discuss) |
| `in-progress` | Agent currently working on this stage |
| `complete` | Stage finished successfully |
| `waiting-user` | Stage 7 User Mode only — waiting for user to complete tasks |
| `blocked` | Cannot proceed due to failed gate check |

### Summary Field

Stages 5, 7, and 8 include a `summary` string. The producing agent writes this when marking the stage complete — 2–3 sentences that capture the essence of the deliverable. Downstream agents should read `status.json` summaries first and only open the full document when they need detail beyond what the summary provides.

| Stage | Who writes | What to capture |
|-------|-----------|-----------------|
| `4-tech-spec` | Hephaestus | Key architectural decisions, component count, file count |
| `6-test-plan` | Artemis | Total test cases, P0 coverage fraction, key risk areas |
| `7-implementation` | Ares | Files created/modified, tests written, deviations |

### Review Verdicts

| Agent | Field | Values | Meaning |
|-------|-------|--------|---------|
| Athena (stage 2) | `verdict` | `approved` / `revisions` | PRD quality assessment |
| Apollo (stage 5) | `verdict` | `sound` / `concerns` / `unsound` | Architecture quality |
| Hera (stage 8) | `alignment_verdict` | `aligned` / `gaps` / `misaligned` | PRD coverage |
| Hermes (stage 9) | `code_review_verdict` | `approved` / `changes-required` | Code quality |
| Cassandra (stage 9) | `risk_verdict` | `clear` / `caution` / `blocked` | Risk assessment |

### Verdict Thresholds

**Apollo verdicts:**
- **Sound**: No critical or high-severity issues found
- **Concerns**: 1-3 high-severity issues that are resolvable with minor spec changes
- **Unsound**: 4+ high-severity issues OR fundamental architectural mismatch with requirements

**Cassandra verdicts:**
- **Clear**: No CRITICAL/HIGH findings, fewer than 3 MEDIUM findings
- **Caution**: 1-3 HIGH findings OR 3+ MEDIUM findings, all addressable
- **Blocked**: Any CRITICAL finding OR 4+ HIGH findings

**Hermes verdicts:**
- **Approved**: No BLOCKER items, all WARNING items acknowledged
- **Changes Required**: Any BLOCKER item OR 3+ unaddressed WARNING items

### History Entry

Each significant pipeline event is appended to the `history` array. This provides an audit trail for Clio and recall commands.

### check_failures (per-stage)

Each stage object in `pipeline` may optionally contain a `check_failures` array. This array is populated by `kratos check --verify` when the retry limit is exhausted for that stage. It is **append-only and never pruned** — entries accumulate as an audit trail.

**Field definitions:**

| Field | Type | Description |
|-------|------|-------------|
| `check_failures` | `[]object` | Optional array on each stage object. Empty or absent means no verification failures recorded. |
| `check_failures[].timestamp` | ISO8601 string | When the failure was recorded |
| `check_failures[].tier` | integer | Verification tier level (1 = file existence, 2 = build/test, 3 = checklist) |
| `check_failures[].checks_failed` | `[]string` | Human-readable list of which checks failed |
| `check_failures[].retries_exhausted` | boolean | Always `true` when recorded (failures only recorded on max retry) |

**Example:**

```json
{
  "pipeline": {
    "5-spec-review-sa": {
      "status": "complete",
      "assignee": "apollo",
      "document": "spec-review-sa.md",
      "check_failures": [
        {
          "timestamp": "2026-03-25T10:00:00+08:00",
          "tier": 1,
          "checks_failed": ["spec-review-sa.md not found or empty"],
          "retries_exhausted": true
        }
      ]
    }
  }
}
```

---

## Agent Update Responsibilities

| Agent | Reads | Updates |
|-------|-------|---------|
| Kratos | All fields | `stage`, `pipeline_status`, `updated`, `history` |
| Athena | `stage`, stage status | Stage 1, 2 status + verdict |
| Hephaestus | `stage`, PRD version | Stage 4 status + `based_on_prd_version` + `summary` |
| Apollo | `stage` | Stage 5 status + verdict |
| Artemis | `stage` | Stage 6 status + `summary` |
| Daedalus | `stage` | Stage 3 status + `output_targets` |
| Ares | `stage`, `implementation_mode` | Stage 7 status + tasks array + `summary` |
| Hera | `stage` | Stage 8 `alignment_verdict` + coverage fields |
| Hermes | `stage` | Stage 9 `code_review_verdict` |
| Cassandra | `stage` | Stage 9 `risk_verdict` |

---

## Conflict Detection

A **stale conflict** exists when:
- `pipeline["4-tech-spec"].based_on_prd_version` < `pipeline["1-prd"].completed`
  (Tech spec was written against an older PRD)

A **gate failure** exists when:
- Target stage prerequisites are not `complete`
- Example: Cannot start stage 4 if stage 2 verdict is not `approved`

---

*Referenced by all agents. See `<KRATOS_ROOT>/references/agent-protocol.md` for document creation procedures.*

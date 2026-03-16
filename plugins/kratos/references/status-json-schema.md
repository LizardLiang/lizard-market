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
  "current_stage": "<stage-id>",
  "pipeline_status": "in-progress | complete | blocked | abandoned",
  "mode": "normal | eco | power",
  "implementation_mode": "ares | user | null",

  "stages": {
    "0-research": {
      "status": "skipped | in-progress | complete",
      "agent": "metis",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["project-overview.md", "tech-stack.md", "architecture.md", "file-structure.md", "conventions.md"]
    },
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
      "agent": "athena",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["prd-review.md"],
      "verdict": "approved | revisions"
    },
    "2.5-decomposition": {
      "status": "skipped | in-progress | complete",
      "agent": "daedalus",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["decomposition.md"],
      "output_targets": ["local", "notion", "linear"]
    },
    "3-tech-spec": {
      "status": "pending | in-progress | complete",
      "agent": "hephaestus",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["tech-spec.md"],
      "based_on_prd_version": "<ISO8601 of prd.md last modified>"
    },
    "4-spec-review-pm": {
      "status": "pending | in-progress | complete",
      "agent": "athena",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["spec-review-pm.md"],
      "verdict": "approved | revisions"
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
      "documents": ["test-plan.md"]
    },
    "7-implementation": {
      "status": "pending | in-progress | complete | waiting-user",
      "agent": "ares",
      "mode": "ares | user",
      "started": "<ISO8601>",
      "completed": "<ISO8601>",
      "documents": ["implementation-notes.md"],
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
| `current_stage` | string | yes | Current active stage ID (e.g., "3-tech-spec") |
| `pipeline_status` | enum | yes | Overall pipeline status |
| `mode` | enum | yes | Execution mode (affects model assignments) |
| `implementation_mode` | enum | no | Set at stage 7; "ares" (AI implements) or "user" (task files created) |

### Stage Status Values

| Status | Meaning |
|--------|---------|
| `pending` | Not yet started, waiting for prerequisites |
| `skipped` | Intentionally bypassed (research, decomposition) |
| `in-progress` | Agent currently working on this stage |
| `complete` | Stage finished successfully |
| `waiting-user` | Stage 7 User Mode only — waiting for user to complete tasks |
| `blocked` | Cannot proceed due to failed gate check |

### Review Verdicts

| Agent | Field | Values | Meaning |
|-------|-------|--------|---------|
| Athena (stage 2) | `verdict` | `approved` / `revisions` | PRD quality assessment |
| Athena (stage 4) | `verdict` | `approved` / `revisions` | Tech spec aligns with PRD |
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

---

## Agent Update Responsibilities

| Agent | Reads | Updates |
|-------|-------|---------|
| Kratos | All fields | `current_stage`, `pipeline_status`, `updated`, `history` |
| Athena | `current_stage`, stage status | Stage 1, 2, 4 status + verdict |
| Hephaestus | `current_stage`, PRD version | Stage 3 status + `based_on_prd_version` |
| Apollo | `current_stage` | Stage 5 status + verdict |
| Artemis | `current_stage` | Stage 6 status |
| Daedalus | `current_stage` | Stage 2.5 status + `output_targets` |
| Ares | `current_stage`, `implementation_mode` | Stage 7 status + tasks array |
| Hera | `current_stage` | Stage 8 `alignment_verdict` + coverage fields |
| Hermes | `current_stage` | Stage 9 `code_review_verdict` |
| Cassandra | `current_stage` | Stage 9 `risk_verdict` |

---

## Conflict Detection

A **stale conflict** exists when:
- `stages["3-tech-spec"].based_on_prd_version` < `stages["1-prd"].completed`
  (Tech spec was written against an older PRD)

A **gate failure** exists when:
- Target stage prerequisites are not `complete`
- Example: Cannot start stage 3 if stage 2 verdict is not `approved`

---

*Referenced by all agents. See `plugins/kratos/references/agent-protocol.md` for document creation procedures.*

# Agent Handoff Specification

Defines what each agent produces, what each consults, and which upstream agent owns each artifact when a prerequisite is missing.

---

## Pipeline Flow

```
Metis -> Athena -> Nemesis -> [Daedalus] -> [Themis] -> Hephaestus -> Apollo -> Artemis -> Ares -> Hera -> Hermes+Cassandra
```

---

## Artifact Ownership

If an agent needs one of these files and it is missing, the agent should stop and tell Kratos to summon the owner.

| Artifact | Owned by | Stage |
|---|---|---|
| `prd.md` | Athena | 1-prd |
| `prd-challenge.md` | Nemesis | 2-prd-review |
| `decomposition.md` | Daedalus | 3-decomposition |
| `context.md` | Themis | 4-discuss |
| `tech-spec.md` | Hephaestus | 5-tech-spec |
| `spec-review-pm.md` | Athena | 6-spec-review-pm |
| `spec-review-sa.md` | Apollo | 7-spec-review-sa |
| `test-plan.md` | Artemis | 8-test-plan |
| `implementation-notes.md` | Ares | 9-implementation |
| `code-review.md` | Hermes | 11-review |
| `risk-analysis.md` | Cassandra | 11-review |

---

## Agent Input/Output Contract

### Metis (Stage 0 - Research)

| | Details |
|---|---|
| **Inputs** | Project root directory |
| **Outputs** | 5 Arena documents in `.claude/.Arena/` |
| **Output format** | Markdown with YAML frontmatter (see arena-templates.md) |
| **Required by** | All downstream agents (optional but enriching) |
| **If missing** | Other agents proceed without Arena context; they scan codebase directly but less efficiently |

### Athena (Stage 1 - PRD Creation)

| | Details |
|---|---|
| **Inputs** | User requirements, clarified answers from Kratos, Arena context (if exists) |
| **Outputs** | `prd.md` in feature folder |
| **Output format** | Markdown following `templates/prd-template.md` |
| **Required by** | Stage 2 (PRD Review), Stage 5 (Tech Spec), Stage 6 (PM Review) |
| **If missing** | Pipeline cannot proceed past Stage 1 |

### Nemesis (Stage 2 - PRD Review)

| | Details |
|---|---|
| **Inputs** | `prd.md` in feature folder |
| **Outputs** | `prd-challenge.md` with verdict |
| **Output format** | Markdown following adversarial review format |
| **Required by** | Stage 5 gate check |
| **Verdict values** | `approved` -> proceed, `revisions` -> loop back to Stage 1 |

### Daedalus (Stage 3 - Decomposition, Optional)

| | Details |
|---|---|
| **Inputs** | Approved `prd.md` |
| **Outputs** | `decomposition.md` (and optionally Notion/Linear items) |
| **Output format** | Markdown following `templates/decomposition-template.md` |
| **Required by** | No hard dependency — enriches Hephaestus, Artemis, Ares, Hermes |
| **If missing** | Downstream agents organize work by natural module boundaries instead of phases |

### Themis (Stage 4 - Discuss, Optional)

| | Details |
|---|---|
| **Inputs** | Approved `prd.md`, codebase patterns, prior `context.md` files from other features |
| **Outputs** | `context.md` with locked implementation decisions |
| **Output format** | Markdown with tagged sections (`<domain>`, `<decisions>`, `<canonical_refs>`, `<code_context>`, `<specifics>`, `<deferred>`) |
| **Required by** | No hard dependency — consumed by Hephaestus if present |
| **If missing** | Hephaestus proceeds without locked decisions — makes reasonable architectural assumptions |

### Hephaestus (Stage 5 - Tech Spec)

| | Details |
|---|---|
| **Inputs** | Approved `prd.md`, `decomposition.md` (if exists), `context.md` (if exists), Arena context |
| **Outputs** | `tech-spec.md` |
| **Output format** | Markdown following `templates/tech-spec-template.md` |
| **Required by** | Stages 6, 7, 8, 9 |
| **If missing** | Pipeline cannot proceed past Stage 5 |
| **context.md note** | If `context.md` exists, Hephaestus reads `<decisions>` and `<canonical_refs>` before speccing — these are locked choices that must not be deviated from without explicit note |

### Apollo (Stage 7 - SA Spec Review)

| | Details |
|---|---|
| **Consult when needed** | Use `status.json` for stage state and summary, `prd.md` for requirements, `tech-spec.md` for design detail when the summary is insufficient, and Arena/codebase only for targeted verification |
| **If a needed file is missing** | Stop and tell Kratos to summon the owning agent from the Artifact Ownership table |
| **Outputs** | `spec-review-sa.md` with verdict |
| **Output format** | Markdown following `templates/spec-review-sa-template.md` |
| **Verdict values** | `sound` / `concerns` / `unsound` |
| **If `unsound`** | Loop back to Stage 5 for Hephaestus revision |
| **If `concerns`** | Proceed with noted concerns; Ares addresses them during implementation |

### Artemis (Stage 8 - Test Plan)

| | Details |
|---|---|
| **Consult when needed** | Use `status.json` for stage state and summary, `prd.md` for requirements, both spec reviews for known concerns, `tech-spec.md` for technical behavior when needed, and `decomposition.md` for phase-aware planning |
| **If a needed file is missing** | Stop and tell Kratos to summon the owning agent from the Artifact Ownership table |
| **Outputs** | `test-plan.md` |
| **Output format** | Markdown following `templates/test-plan-template.md` |
| **Required by** | Stage 9 (Ares references for test writing) |
| **Test level guidance** | Unit for logic, integration for components, E2E for user workflows |

### Ares (Stage 9 - Implementation)

| | Details |
|---|---|
| **Consult when needed** | Use `status.json` for stage state and summaries, `test-plan.md` for verification goals, `tech-spec.md` for implementation detail when summaries are insufficient, `prd.md` for requirement context, `decisions.md` for rationale, and `decomposition.md` for task sequencing |
| **If a needed file is missing** | Stop and tell Kratos to summon the owning agent from the Artifact Ownership table |
| **Outputs (Ares Mode)** | Implementation code + `implementation-notes.md` |
| **Outputs (User Mode)** | `tasks/*.md` + `tasks/00-overview.md` |
| **Output format** | Code files + markdown notes following `templates/implementation-notes-template.md` |
| **Required by** | Stage 10 (PRD alignment) |

### Hera (Stage 10 - PRD Alignment)

| | Details |
|---|---|
| **Inputs** | `prd.md`, `test-plan.md`, `implementation-notes.md`, test files in codebase |
| **If a needed file is missing** | Stop and tell Kratos to summon the owning agent from the Artifact Ownership table |
| **Outputs** | `prd.md` — Section 10 (Alignment) updated with checkboxes and verdict |
| **Output format** | Edits `## 10. Alignment` section in prd.md with criterion table and verdict |
| **Verdict values** | `aligned` / `gaps` / `misaligned` |
| **If `aligned`** | Proceed to stage 11 (Hermes + Cassandra) |
| **If `gaps`** | Return to stage 9 (Ares) to add missing test coverage |
| **If `misaligned`** | Block pipeline — escalate to user, fundamental scope issue |

### Hermes (Stage 11 - Code Review)

| | Details |
|---|---|
| **Consult when needed** | Use `status.json` for stage state and summaries, `implementation-notes.md` for delivered work, `test-plan.md` for expected coverage, `prd.md` for requirement alignment, `tech-spec.md` for intended design when needed, and `decomposition.md` for phase verification |
| **If a needed file is missing** | Stop and tell Kratos to summon the owning agent from the Artifact Ownership table |
| **Outputs** | `code-review.md` with verdict |
| **Output format** | Markdown following `templates/code-review-template.md` |
| **Verdict values** | `approved` / `changes-required` |
| **Rule sources** | `rules/default.md`, `rules/<language>.md`, `.claude/.Arena/review-rules/conventions.md` |
| **If rule files missing** | Use built-in Greatness Hierarchy as fallback (see hermes.md) |

### Cassandra (Stage 11 - Risk Analysis)

| | Details |
|---|---|
| **Consult when needed** | Use `status.json` for stage state and summaries, inspect implementation code and git diff as the primary risk surface, and open `tech-spec.md` only when intended architecture or contract detail matters to a risk |
| **If a needed file is missing** | Stop and tell Kratos to summon the owning agent from the Artifact Ownership table |
| **Outputs** | `risk-analysis.md` with verdict |
| **Output format** | Markdown following `templates/risk-analysis-template.md` |
| **Verdict values** | `clear` / `caution` / `blocked` |

---

## General Fallback Rules

1. **Arena not found**: Agent scans codebase directly. Less efficient but functional.
2. **Decomposition not found**: Agent organizes by natural module boundaries.
3. **Context not found**: Hephaestus makes reasonable architectural assumptions (no locked decisions to follow).
4. **Template not found**: Agent uses the format described in its own instructions.
5. **Rule files not found**: Agent uses built-in defaults (Hermes: Greatness Hierarchy).
6. **Status.json malformed**: Agent reports error to Kratos; Kratos re-initializes status.json.

---

## Document Creation

All agents create documents using the Write tool. Agents have Write access in their tool list. The Task tool prompt should include:
- `FOLDER: .claude/feature/<name>/` — the target directory
- `CRITICAL: You are responsible for producing <name>.md before completing.`

Kratos validates required deliverables after the agent finishes.

---

*See `status-json-schema.md` for the complete status.json specification.*

# Agent Handoff Specification

Defines what each agent produces, what each expects as input, and how to handle missing prerequisites.

---

## Pipeline Flow

```
Metis -> Athena -> Athena(review) -> [Daedalus] -> [Themis] -> Hephaestus -> Athena+Apollo -> Artemis -> Ares -> Hera -> Hermes+Cassandra
```

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

### Athena (Stage 2 - PRD Review)

| | Details |
|---|---|
| **Inputs** | `prd.md` in feature folder |
| **Outputs** | `prd-review.md` with verdict |
| **Output format** | Markdown following `templates/prd-review-template.md` |
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

### Athena (Stage 6 - PM Spec Review)

| | Details |
|---|---|
| **Inputs** | `tech-spec.md`, `prd.md` |
| **Outputs** | `spec-review-pm.md` with verdict |
| **Output format** | Markdown following `templates/prd-review-template.md` |
| **Verdict values** | `approved` -> proceed, `revisions` -> loop back to Stage 5 |

### Apollo (Stage 7 - SA Spec Review)

| | Details |
|---|---|
| **Inputs** | `tech-spec.md`, Arena context, codebase patterns |
| **Outputs** | `spec-review-sa.md` with verdict |
| **Output format** | Markdown following `templates/spec-review-sa-template.md` |
| **Verdict values** | `sound` / `concerns` / `unsound` |
| **If `unsound`** | Loop back to Stage 5 for Hephaestus revision |
| **If `concerns`** | Proceed with noted concerns; Ares addresses them during implementation |

### Artemis (Stage 8 - Test Plan)

| | Details |
|---|---|
| **Inputs** | `prd.md`, `tech-spec.md`, `decomposition.md` (if exists), codebase test conventions |
| **Outputs** | `test-plan.md` |
| **Output format** | Markdown following `templates/test-plan-template.md` |
| **Required by** | Stage 9 (Ares references for test writing) |
| **Test level guidance** | Unit for logic, integration for components, E2E for user workflows |

### Ares (Stage 9 - Implementation)

| | Details |
|---|---|
| **Inputs** | `tech-spec.md`, `test-plan.md`, `decomposition.md` (if exists) |
| **Outputs (Ares Mode)** | Implementation code + `implementation-notes.md` |
| **Outputs (User Mode)** | `tasks/*.md` + `tasks/00-overview.md` |
| **Output format** | Code files + markdown notes following `templates/implementation-notes-template.md` |
| **Required by** | Stage 10 (PRD alignment) |

### Hera (Stage 10 - PRD Alignment)

| | Details |
|---|---|
| **Inputs** | `prd.md`, `test-plan.md`, `implementation-notes.md`, test files in codebase |
| **Outputs** | `prd-alignment.md` with verdict |
| **Output format** | Markdown with criterion-to-test mapping table |
| **Verdict values** | `aligned` / `gaps` / `misaligned` |
| **If `aligned`** | Proceed to stage 11 (Hermes + Cassandra) |
| **If `gaps`** | Return to stage 9 (Ares) to add missing test coverage |
| **If `misaligned`** | Block pipeline — escalate to user, fundamental scope issue |

### Hermes (Stage 11 - Code Review)

| | Details |
|---|---|
| **Inputs** | All feature documents + implementation code (git diff) |
| **Outputs** | `code-review.md` with verdict |
| **Output format** | Markdown following `templates/code-review-template.md` |
| **Verdict values** | `approved` / `changes-required` |
| **Rule sources** | `rules/default.md`, `rules/<language>.md`, `.claude/.Arena/review-rules/conventions.md` |
| **If rule files missing** | Use built-in Greatness Hierarchy as fallback (see hermes.md) |

### Cassandra (Stage 11 - Risk Analysis)

| | Details |
|---|---|
| **Inputs** | Implementation code (git diff), `tech-spec.md` |
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
- `CRITICAL: You MUST create the file <name>.md before completing.`

Agents verify file creation by reading it back before reporting completion.

---

*See `status-json-schema.md` for the complete status.json specification.*

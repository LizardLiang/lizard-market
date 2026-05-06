# Agent Handoff Specification

This document defines the interface and deliverables for every agent in the Kratos pipeline, plus which upstream agent owns each artifact when a prerequisite is missing.

---

## The Pipeline

| Stage | Agent | Deliverable | Key Prerequisite |
|-------|-------|-------------|------------------|
| 1 | Athena | `prd.md` | Requirements (from user) |
| 2 | Nemesis | `prd-challenge.md` | `prd.md` |
| 3 | Daedalus | `decomposition.md` | `prd.md` (vetted) |
| 4 | Themis | `context.md` | `prd.md` |
| 5 | Hephaestus | `tech-spec.md` | `prd.md` |
| 6 | Apollo | `spec-review-sa.md` | `tech-spec.md` |
| 7 | Artemis | `test-plan.md` | `tech-spec.md` |
| 8 | Ares | `implementation-notes.md` | `test-plan.md` |
| 9 | Hera | `prd-alignment.md` | implementation |
| 10 | Hermes | `code-review.md` | implementation |
| 10 | Cassandra | `risk-analysis.md` | implementation |

---

## Artifact Ownership

If an agent needs one of these files and it is missing, the agent should stop and tell Kratos to summon the owner.

| Artifact | Owned by | Stage |
|---|---|---|
| `prd.md` | Athena | 1-prd |
| `prd-challenge.md` | Nemesis | 2-prd-review |
| `decomposition.md` | Daedalus | 3-decomposition |
| `tech-spec.md` | Hephaestus | 4-tech-spec |
| `spec-review-sa.md` | Apollo | 5-spec-review-sa |
| `test-plan.md` | Artemis | 6-test-plan |
| `implementation-notes.md` | Ares | 7-implementation |
| `prd-alignment.md` | Hera | 8-prd-alignment |
| `code-review.md` | Hermes | 9-review |
| `risk-analysis.md` | Cassandra | 9-review |

---

## Agent Missions

### Athena (Product Manager)
- **Mission 1**: Create `prd.md` based on user requirements.
- **Mission 2**: Review `prd.md` for quality (Stage 2).

### Nemesis (Adversary)
- **Mission**: Challenge `prd.md` from devil's advocate and user advocate perspectives.

### Daedalus (Architect)
- **Mission**: Decompose the feature into logical, verifiable phases in `decomposition.md`.

### Themis (Judge)
- **Mission**: Surface implementation choices and lock decisions in `context.md`.

### Hephaestus (Tech Lead)
- **Mission**: Create `tech-spec.md` based on the approved PRD.

### Apollo (Architecture Review)
- **Mission**: Review `tech-spec.md` for technical soundness (Stage 5).
- **Consult when needed**: Use `status.json` for stage state and summary, `prd.md` for requirements, `tech-spec.md` for design detail when the summary is insufficient, and Arena/codebase only for targeted verification.
- **If a needed file is missing**: Stop and tell Kratos to summon the owning agent from the Artifact Ownership table.

### Artemis (QA Lead)
- **Mission**: Create `test-plan.md` mapping requirements to test cases (Stage 6).
- **Consult when needed**: Use `status.json` for stage state and summary, `prd.md` for requirements, `spec-review-sa.md` for known concerns, `tech-spec.md` for technical behavior when needed, and `decomposition.md` for phase-aware planning.
- **If a needed file is missing**: Stop and tell Kratos to summon the owning agent from the Artifact Ownership table.

### Ares (Implementation)
- **Mission**: Implement the feature in code and write tests (Stage 7).
- **Consult when needed**: Use `status.json` for stage state and summaries, `test-plan.md` for verification goals, `tech-spec.md` for implementation detail when summaries are insufficient, `prd.md` for requirement context, `decisions.md` for rationale, and `decomposition.md` for task sequencing.
- **If a needed file is missing**: Stop and tell Kratos to summon the owning agent from the Artifact Ownership table.

### Hera (Alignment)
- **Mission**: Verify that implementation matches PRD requirements (Stage 8).
- **Inputs**: `prd.md`, `test-plan.md`, `implementation-notes.md`, test files in codebase.
- **If a needed file is missing**: Stop and tell Kratos to summon the owning agent from the Artifact Ownership table.

### Hermes (Code Review)
- **Mission**: Review implementation for quality and conventions (Stage 9).
- **Consult when needed**: Use `status.json` for stage state and summaries, `implementation-notes.md` for delivered work, `test-plan.md` for expected coverage, `prd.md` for requirement alignment, `tech-spec.md` for intended design when needed, and `decomposition.md` for phase verification.
- **If a needed file is missing**: Stop and tell Kratos to summon the owning agent from the Artifact Ownership table.

### Cassandra (Risk Analyst)
- **Mission**: Identify security, breaking change, and reliability risks (Stage 9).
- **Consult when needed**: Use `status.json` for stage state and summaries, inspect implementation code and git diff as the primary risk surface, and open `tech-spec.md` only when intended architecture or contract detail matters to a risk.
- **If a needed file is missing**: Stop and tell Kratos to summon the owning agent from the Artifact Ownership table.

---

## Deliverable Quality Standards

### prd.md (Athena)
- [ ] Explicit problem statement
- [ ] User personas defined
- [ ] Measurable success metrics (not just "improved UX")
- [ ] Numbered requirements with clear acceptance criteria
- [ ] Scope boundary (In-scope / Out-of-scope)

### prd-challenge.md (Nemesis)
- [ ] Verdict (Approved / Revisions / Rejected)
- [ ] Rationale for verdict
- [ ] List of "Sharp Edges" (potential issues)
- [ ] User advocacy findings

### decomposition.md (Daedalus)
- [ ] Feature broken into logical, ordered phases
- [ ] Each phase has clear, verifiable tasks
- [ ] Dependencies between phases identified

### context.md (Themis)
- [ ] List of implementation decisions made
- [ ] Alternatives considered and rejected
- [ ] Justification for each choice

### tech-spec.md (Hephaestus)
- [ ] Architectural overview
- [ ] Data model / Schema changes
- [ ] API design / Method signatures
- [ ] Implementation plan (step-by-step)
- [ ] Non-functional considerations (performance, security)

### spec-review-sa.md (Apollo)
- [ ] Verdict (Sound / Concerns / Unsound)
- [ ] Rationale for verdict
- [ ] Security assessment
- [ ] Performance assessment

### test-plan.md (Artemis)
- [ ] Unit test cases
- [ ] Integration test cases
- [ ] Acceptance criteria mapping (Requirement ID -> Test Case)
- [ ] Test environment/data requirements

### implementation-notes.md (Ares)
- [ ] Summary of what was built
- [ ] List of created/modified files
- [ ] List of tests written and results
- [ ] Any deviations from the tech spec (with justification)

### prd-alignment.md (Hera)
- [ ] Verdict (Aligned / Gaps / Misaligned)
- [ ] Requirement-by-requirement verification status
- [ ] Test coverage percentage

### code-review.md (Hermes)
- [ ] Verdict (Approved / Changes Required)
- [ ] Tiered findings (Correctness, Safety, Clarity, etc.)
- [ ] Specific file:line references
- [ ] Proposed fixes

### risk-analysis.md (Cassandra)
- [ ] Verdict (Clear / Caution / Blocked)
- [ ] Categorized risks (Security, Correctness, Reliability, etc.)
- [ ] Severity ratings
- [ ] Recommended mitigations

---

## Communication Protocol

1. **State your mission** clearly at the start of every session.
2. **Auto-discover the feature** by searching `.claude/feature/`.
3. **Consult only the documents you need** for the current decision.
4. **If a needed file is missing**, stop and report the owning upstream agent to Kratos.
5. **Update status.json** after every successful completion.
6. **Use real timestamps** from the Kratos CLI (`~/.kratos/bin/kratos now`) when direct writes are required.
7. **Report gate status** to Kratos (Passed / Blocked).

Kratos validates required deliverables after the agent finishes.

---

*See `plugins/kratos/references/status-json-schema.md` for status.json details.*

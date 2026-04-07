# Agent Handoff Specification

This document defines the interface and deliverables for every agent in the Kratos pipeline. All agents MUST follow these contracts to ensure the pipeline remains verifiable and reliable.

---

## The Pipeline

| Stage | Agent | Deliverable | Key Prerequisite |
|-------|-------|-------------|------------------|
| 1 | Athena | prd.md | Requirements (from user) |
| 2 | Nemesis | prd-challenge.md | prd.md |
| 3 | Daedalus | decomposition.md | prd.md (vetted) |
| 4 | Themis | context.md | prd.md |
| 5 | Hephaestus| tech-spec.md | prd.md |
| 6 | Apollo | spec-review-sa.md | tech-spec.md |
| 7 | Artemis | test-plan.md | tech-spec.md |
| 8 | Ares | impl-notes.md | test-plan.md |
| 9 | Hera | prd-alignment.md | implementation |
| 10 | Hermes | code-review.md | implementation |
| 10 | Cassandra | risk-analysis.md | implementation |

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
- **Mission**: Review `tech-spec.md` for technical soundness (Stage 6).

### Artemis (QA Lead)
- **Mission**: Create `test-plan.md` mapping requirements to test cases (Stage 7).

### Ares (Implementation)
- **Mission**: Implement the feature in code and write tests (Stage 8).

### Hera (Alignment)
- **Mission**: Verify that implementation matches PRD requirements (Stage 9).

### Hermes (Code Review)
- **Mission**: Review implementation for quality and conventions (Stage 10).

### Cassandra (Risk Analyst)
- **Mission**: Identify security, breaking change, and reliability risks (Stage 10).

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
3. **Read prerequisites** before writing your own deliverable.
4. **Update status.json** after every successful completion.
5. **Use real timestamps** from the Kratos CLI (`~/.kratos/bin/kratos now`).
6. **Report gate status** to Kratos (Passed / Blocked).

*See `plugins/kratos/references/status-json-schema.md` for status.json details.*

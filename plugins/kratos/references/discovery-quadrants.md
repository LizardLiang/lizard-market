# Discovery Quadrants — the Quadrant Sweep

Shared protocol for Athena's gap analysis (`pipeline/gap-analysis.md`, command-mode Athena) and Odysseus's facet decomposition. Checklists and facet enumeration only find gaps you already know to look for — **known unknowns**. This sweep hunts the two dark quadrants — tacit assumptions (**unknown knowns**) and blind spots (**unknown unknowns**) — and converts every discovery into a branch in your gap/facet tree, so the existing readiness gates cover them.

Run it **once**, after enumerating checklist gaps / facets and **before** scoring readiness. Time-box it: each technique is one focused pass, not a research project.

## The four quadrants

| Quadrant | What it is | How it's covered |
|----------|-----------|------------------|
| Known knowns | Facts you can cite | Evidence check (§1) |
| Known unknowns | Questions already on the table | Your gap/facet tree + question loop |
| Unknown knowns | Tacit assumptions — yours, the user's, the repo's | Assumption surfacing (§2) |
| Unknown unknowns | Risks nobody has thought to ask about | Discovery techniques (§3) |

## §1 Evidence check (known knowns)

For every branch you marked resolved **without asking**, name the evidence: the user's literal words, a repo file, or an Arena shard. No nameable evidence → it was an assumption in disguise; reclassify it as `[open]` or `[assumed: X]`.

## §2 Assumption surfacing (unknown knowns)

- **Yours:** complete the sentence *"If I started writing right now, I would just assume ___"* at least three times. Each completion becomes a branch (`[open]` if it needs the user, `[assumed: X]` with risk-if-wrong if you're deciding it yourself).
- **The user's:** restate the request in different words and look for a second plausible reading. If two readings diverge materially, that divergence is your highest-priority question — ask it before anything else.
- **The repo's:** conventions or constraints the code enforces that nobody stated — Arena `constraints.md`, error-handling/auth/tenancy/naming patterns in neighboring code. Silently violating one is the classic unknown known; note each relevant one as evidence on the branch it settles.

## §3 Discovery techniques (unknown unknowns)

You cannot list unknown unknowns directly — you run techniques that convert them into known unknowns. Run **all six**; each generates branches from outside the checklist:

1. **Premortem.** *"It's three months after ship and this feature failed badly."* Write the 3 most plausible post-mortem headlines. Any headline whose cause has no branch in your tree → new `[open]` branch.
2. **Inversion.** Take your 3 most load-bearing assumptions (from §2) and ask *"what breaks if this is false?"* A consequential break with no mitigation → branch.
3. **Boundary probe.** Walk the standard perturbations against this specific feature: ×10 scale, zero/empty state, concurrent/duplicate action, hostile actor, partial failure mid-operation, time (timeout, retry, clock skew). Record only the ones that produce a real, feature-specific question — not generic hand-waving.
4. **Actor sweep.** List every actor who touches this: each end-user role, admin, attacker, support, ops/on-call, the next developer, external systems. An actor with zero branches in your tree is a blind spot — add one.
5. **Analogous failures.** How have others gotten *this kind of feature* wrong? **Athena:** put it to Mimir — "common failure modes, pitfalls, and things teams regret when building X". **Odysseus:** mine the repo — `git log --oneline` for reverts/fixes in the target area, `grep -rn "TODO\|FIXME"` there, and existing tests for edge cases someone already paid for.
6. **Checklist escape.** Name 2 candidate gaps NOT covered by any checklist item or facet you already have. If after honestly trying you can't, say so in the ledger — the point is that the checklist is a floor, not a ceiling.

A technique that surfaces nothing is a legitimate result — **but only after actually running it**. "Nothing surfaced" without the technique's intermediate output (the headlines, the actor list, the two candidate gaps) is a skipped step, not a clean pass.

## §4 The Discovery Ledger (required artifact)

Summarize the sweep in this table. It travels with your deliverable (gap analysis → `CLARIFIED_REQUIREMENTS` → PRD appendix; Odysseus → tactical plan), and the readiness gates (WRITE_READY / PLAN_READY) require it.

```markdown
## Discovery Ledger

| Quadrant | Findings |
|----------|----------|
| Known knowns | <N branches resolved with named evidence; M reclassified by evidence check> |
| Known unknowns | <N branches asked / assumed / out-of-scope> |
| Unknown knowns | <assumptions surfaced and their disposition> |
| Unknown unknowns | premortem: <headlines → branches> · inversion: <...> · boundary: <...> · actors: <...> · analogous: <...> · escape: <...> |
```

Every branch the sweep generates enters the normal question loop — asked, `[assumed: X]`, or `[out of scope]`. The sweep never bypasses the gates; it feeds them.

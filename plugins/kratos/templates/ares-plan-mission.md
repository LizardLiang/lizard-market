# Ares Mission — Implement an Approved Tactical Plan

Fetched by the orchestrator (Iris, Odysseus on approval, `/kratos:quick`) with `<kratos-bin> template get ares-plan-mission`. Fill the angle-bracket fields and pass the block as the `prompt` of `Task(subagent_type: "kratos:ares", mode: "acceptEdits")`. Nothing else needs to be read at approval time — not this plan, not quick.md.

```
MISSION: Implement Approved Tactical Plan
PLAN: <absolute path of .claude/.Arena/tactical-plans/<slug>.md in the main checkout — never a worktree's copy>
PHASE: <n of m — only when the plan has a ## Phases table; omit otherwise>
DONE: <none | git log --oneline <base>..HEAD — only with PHASE>
WORKTREE: <absolute path — only when the work runs in a worktree other than the main checkout>
ORIGINAL_USER_REQUEST: <the user's words, verbatim — the scope contract>
APPROVAL_NOTES: <requirements the user added with the approval, verbatim, or none>
TICKET: <#N or none>
REQUIREMENTS: Read the plan file first and treat it as the execution contract. Refuse it if its frontmatter says status: draft. If the plan is missing, ambiguous, or contradicts the repo, stop and report the mismatch before editing. Implement only the PHASE named above when one is given; verify with that phase's Verify command, commit, and hand back an ARES PHASE CHECKPOINT. Verification is proportional: the plan's Validation section and the tests covering your files; a full e2e or browser suite only when the plan names it. Before any edit, follow your INTENTION protocol. Each test you write for a spec-delta scenario (`.claude/feature/<slug>/spec-delta/*.md`) carries that scenario's title in its test name, so the alignment check can match them mechanically.
```

Rules the orchestrator keeps on its side:

- `run:` in the plan frontmatter sets the phase cadence — `all-phases` chains every phase with one progress line per checkpoint; `per-phase` reports and waits. Never ask "continue?" under `all-phases`.
- One review and one fix round per user turn; findings from a second reviewer start their own round — never `SendMessage` them into a running Ares.

After every hand-back, work this ordered list. It replaces the review offer (Post-Task rule 3) and the spec-promotion offer (rule 6) of `commands/quick.md` for plan missions, so the user answers each question once. The ticket note (rule 2) and its question stay as they are.

1. Record `git rev-parse HEAD` as `base` before the first Ares spawn (a plan with a `## Phases` table does this already). Run `<kratos-bin> verify --landed --hash <hash>`, then `TaskStop` the finished Ares id. When verify is BLOCKED or the hand-back is `ARES NEEDS …`, follow `commands/quick.md` Post-Task rule 1 and the clarification rules, and go no further in this list.
2. After the last phase lands, ask exactly ONE `AskUserQuestion`, multiSelect, with these options in this order. The user may pick none.
   - "Alignment check — Hera confirms every scenario in the spec delta has a passing test (Recommended)" — offer this option only when a spec delta (pending or archived) exists for this slug or the plan has Verify commands.
   - "Code review — Hermes"
   - "Archive the spec delta into the living spec" — offer this option only when `<kratos-bin> spec list --changes` shows a pending delta for this slug.
3. Run what the user picked in this order: Hera, then Hermes, then archive. Spawn Hera with `MISSION: Plan Alignment`, `PLAN`, `FEATURE: <slug>`, `RANGE: <base>..<head>` (the commits Ares landed), `WORKTREE` when set, and Ares's RED/GREEN lines. Run archive as `commands/spec-archive.md` § "Offer after implementation" steps 3 to 5.
4. Hera verdict `gaps` or `misaligned`: re-spawn Ares once with the gap list, then run Hera once more. Never loop a third time. Report what is still open. This Ares re-spawn is separate from the one Hermes BLOCKER fix of `commands/quick.md` Post-Task rule 4. When the Hermes fix changes code after an `aligned` verdict, say in one line that the alignment check ran before that fix.
5. Archive picked while no `aligned` verdict exists for this slug: archive anyway and print one line that no alignment check passed.

Ask the review and archive questions only through `AskUserQuestion`, never in prose. Never tell the user to run `/kratos:spec-archive` as a substitute for the question.

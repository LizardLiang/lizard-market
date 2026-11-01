# Ares Mission — Implement an Approved Tactical Plan

Fetched by the orchestrator (Iris, Odysseus on approval, `/kratos:quick`) with `<kratos-bin> template get ares-plan-mission`. Fill the angle-bracket fields and pass the block as the `prompt` of `Task(subagent_type: "kratos:ares", mode: "acceptEdits")`. Nothing else needs to be read at approval time — not this plan, not quick.md.

```
MISSION: Implement Approved Tactical Plan
PLAN: <.claude/.Arena/tactical-plans/<slug>.md>
PHASE: <n of m — only when the plan has a ## Phases table; omit otherwise>
DONE: <none | git log --oneline <base>..HEAD — only with PHASE>
WORKTREE: <absolute path — only when the work runs in a worktree other than the main checkout>
ORIGINAL_USER_REQUEST: <the user's words, verbatim — the scope contract>
APPROVAL_NOTES: <requirements the user added with the approval, verbatim, or none>
TICKET: <#N or none>
REQUIREMENTS: Read the plan file first and treat it as the execution contract. Refuse it if its frontmatter says status: draft. If the plan is missing, ambiguous, or contradicts the repo, stop and report the mismatch before editing. Implement only the PHASE named above when one is given; verify with that phase's Verify command, commit, and hand back an ARES PHASE CHECKPOINT. Verification is proportional: the plan's Validation section and the tests covering your files; a full e2e or browser suite only when the plan names it. Before any edit, follow your INTENTION protocol.
```

Rules the orchestrator keeps on its side:

- `run:` in the plan frontmatter sets the phase cadence — `all-phases` chains every phase with one progress line per checkpoint; `per-phase` reports and waits. Never ask "continue?" under `all-phases`.
- After every hand-back: `<kratos-bin> verify --landed --hash <hash>`, then `TaskStop` the finished Ares id.
- One review and one fix round per user turn; findings from a second reviewer start their own round — never `SendMessage` them into a running Ares.

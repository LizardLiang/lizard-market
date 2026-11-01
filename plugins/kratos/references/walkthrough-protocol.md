# Walkthrough Protocol — live demos, UI mocks, demo guides

Read by Iris (WORK mode) before the first step of a walkthrough, before presenting a UI mock, and before building a demo guide or tutorial. Not injected — loaded on demand so `iris.md` stays under the inline budget. Every rule here is a correction the user made in the week of 2026-10-20 → 10-31; the quotes are theirs.

---

## 1. Walkthrough (the user drives, you guide)

**Setup — before the first hint:**
1. **One backend, one env file.** Every key the flow needs (`INTEGRATION_SECRET_KEY`, seeds off, test flags) in that one file. A second, keyless backend made the token unreadable and drew "what the fuck" (2026-10-24 04:18).
2. **One window per role, nothing stale.** Close every other app window first; a stale operator window on the wrong backend was misdiagnosed as a permission bug for two turns.
3. **Sign the user in yourself** — the role they will act as, on the page the first step needs. "you should sign in for me" (04:06).
4. **Say what will be written where.** Before the first step that writes to an external system (ECOUNT, Mallbic, a vendor API), state it in one line and get a yes. Every record created carries a test marker — `ERP_TEST_*` in name and code, overriding app defaults such as `MB-39`. 2026-10-24 09:27: one click created `MB001` in the live company; cleanup took an hour.
5. **Register every external write** as it happens — `.claude/.Arena/test-data-register.md`, one line: `<date> · <system> · <record id/code> · <delete call or screen>`. Cleanup reads this file; never a memory of what "probably" got created.

**Running it:**
- One hinted step per turn: what to click, what they should see, how to view the result. Wait for them between steps.
- Drive the visible browser window the user is watching (agent-browser headed, their window size) — never API scripts for a step they asked to see.
- A step that fails twice goes to Hades (DIG mode table); do not re-try the same step a third time with a guess.
- "how long" from the user gets a one-line answer from `<scratchpad>/ares-progress.md` or your own state, never a stop.

**Cleanup:**
- Read the register; delete in reverse order of creation; confirm each delete with the system's own list call, not the UI's absence.
- More than ~10 browser actions of cleanup → spawn a `general-purpose` agent with the register and a results file; cleanup run inline flooded the context into a compaction (2026-10-25 10:43).
- Stop any background refresh job before cleanup and restart it after (a running refresh refilled deleted snapshots).

## 2. UI mock (you present, the user judges)

1. **Look before you show.** Screenshot desktop, dark theme and phone width; read the three images yourself; fix what you see. "are you sure you like that toolbar design?" caught an unchecked mock (2026-10-23 11:44); a dark toolbar shipped into the light theme from an unchecked mock (10-24 07:23).
2. **Open it on screen.** `Start-Process <file>` / the browser window — a file path or a screenshot path in chat is "you did not show me shit" (10-24 08:33).
3. **Cold-open test.** Before presenting, answer in one paragraph: a first-time user opens this page — what do they see, what do they do first, where is the setup, how do they redo it. If you cannot, the mock is not ready ("我真的不知道使用者要怎麼使用", 10-26 13:24).
4. **One place per job.** A job editable on two pages, or two cards for one credential, is a design defect, not a feature ("你只需要一個", 10-26 09:43).
5. **Design proposals** with several options are one HTML page with clickable mock screens, not chat text.

## 3. Demo guide, tutorial, PDF

1. **Inventory first.** Enumerate the feature's screens from code — routes, wizards, bulk toolbars, settings — before taking a single screenshot. A guide built from the bug-fix screenshots at hand was redone three times (2026-10-31 02:23 → 04:22: "看起來不像是從最一開始開始講解", "你沒有介紹批次對應").
2. **Start from an empty tenant** and walk the inventory in the order a new user meets it; every screen gets its one-paragraph "what you see, what you do".
3. **One scoping question** when the source images are test artifacts or the audience is unstated: "first-run guide for a new user, or a feature tour for an existing one?" — the one question that would have saved two rounds.
4. **Screenshots in bulk go to a subagent** that returns a verdict list (`file · what it shows · usable yes/no`), or a contact sheet; reading 60 full-size images inline forced a mid-task compaction (10-31 03:44).
5. Restore a minimized browser window before headed screenshots (30 s vs <1 s, and short UI states are missed).

## 4. Environment facts survive compaction

Whatever you stand up — fake servers and their ports, worktrees and their branches, the demo backend command, the restart script — goes into `.claude/.Arena/handoff.md` under `## Environment` the moment it exists (one line each). The compact SessionStart hook re-injects that section; without it the fake Mallbic server was forgotten after a compaction and the user was asked for a real token ("就跟你說要用 dummy server 了", 10-26 10:15).

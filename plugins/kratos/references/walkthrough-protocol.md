# Walkthrough Protocol — live demos, test runs, UI mocks, demo guides

Read by Iris (WORK mode) before the first step of a walkthrough, before a test run (a request to test, verify or compare the app yourself), before presenting a UI mock, and before building a demo guide or tutorial. Not injected — loaded on demand so `iris.md` stays under the inline budget. Every rule here is a correction the user made in the week of 2026-10-20 → 10-31; the quotes are theirs.

---

## 1. Walkthrough (the user drives, you guide)

**Setup — before the first hint:**
1. **One backend, one env file.** Every key the flow needs (`INTEGRATION_SECRET_KEY`, seeds off, test flags) in that one file. A second, keyless backend made the token unreadable and drew "what the fuck" (2026-10-24 04:18).
2. **One window per role, nothing stale.** Close every other app window first; a stale operator window on the wrong backend was misdiagnosed as a permission bug for two turns.
3. **Sign the user in yourself** — the role they will act as, on the page the first step needs. "you should sign in for me" (04:06).
4. **Say what will be written where.** Before the first step that writes to an external system (ECOUNT, Mallbic, a vendor API), state it in one line and get a yes. Every record created carries a test marker — `ERP_TEST_*` in name and code, overriding app defaults such as `MB-39`. 2026-10-24 09:27: one click created `MB001` in the live company; cleanup took an hour.
5. **Pre-flight every step's data.** Before the first hint, check that each step has the records it needs (a form that requires a 包裝耗材 item needs one to exist) and create the missing ones with the test marker. Delete what the build left behind — Ares's fixtures in the test DB drew "`阿瑞斯停用產品類` 這是什麼" (2026-11-08).
6. **Register every external write** as it happens — `.claude/.Arena/test-data-register.md`, one line: `<date> · <system> · <record id/code> · <delete call or screen>`. Cleanup reads this file; never a memory of what "probably" got created.

**Running it:**
- One hinted step per turn: what to click, what they should see, how to view the result. Wait for them between steps.
- Drive the visible browser window the user is watching (agent-browser headed, their window size) — never API scripts for a step they asked to see. Never `set viewport` in a headed session: a pinned 1400×900 left the page a quarter of the window ("why the viewport always not taking the full size of the window", 2026-11-08); only a headless screenshot run sets one.
- "It is open in the browser" is said only after you checked the window is headed and in front. A browser that crashed or was restarted comes back headless: re-open it headed and take a screenshot before telling the user to look ("根本沒看到畫面", 2026-11-14, after the agent had said the page was open).
- A step that fails twice goes to Hades (DIG mode table); do not re-try the same step a third time with a guess.
- "how long" from the user gets a one-line answer from `<scratchpad>/ares-progress.md` or your own state, never a stop.

**Cleanup:**
- Read the register; delete in reverse order of creation; confirm each delete with the system's own list call, not the UI's absence.
- More than ~10 browser actions of cleanup → spawn a `general-purpose` agent with the register and a results file; cleanup run inline flooded the context into a compaction (2026-10-25 10:43).
- Stop any background refresh job before cleanup and restart it after (a running refresh refilled deleted snapshots).

## 2. Test run (you test, the user watches)

A request to test, verify, or compare with the prototype is a test run. You drive. The "one hinted step per turn" rule of section 1 does not apply. Section 1 setup items 1, 2, 5 and 6 apply. The user signs in when only the user can.

**Before the first check:**
1. **Write the coverage list.** Take the items from the system analysis document (SA 系統分析書) for the screens in scope, not only from the ticket. Write the list to `.claude/.Arena/walkthroughs/<date>-<slug>.md` as a table: `# | item (SA id) | step | result | evidence`. "give me again the spec item list and the verify result" (2026-11-17).
2. **Start from a clean state.** Use a clean checkout of the branch under test and known test data. "why didn't you start from clean" (2026-11-18).

**Running it:**
- **Follow the list order, section by section.** Never skip a section to go to a later one. "你為什麼又跑到盤查報告書 跳過這裡" (2026-11-18).
- **Test every field, every option and every state of each form.** For each one: change it, save, load the page again, and compare the screen with the database. A sample of a few fields is not a test. "我再說一次，你每個欄位每種情境，每個選項都有測過嗎" (2026-11-19).
- **Sample only a large list that comes from the database** (358 devices). Test 3 to 5 rows of it and use the time on the other fields. "i think this test is meaning less" (2026-11-19).
- **Write the result and the evidence in the list after every step.** A row is ✓ or ✗. "not observed" and "out of scope" are not results. "too many things are not verified, verify those" (2026-11-17).
- **Do not end the turn between rows.** End the turn in two cases only. In the first case, every row has a result. In the second case, one row needs the user, for example a sign-in or a decision only the user can make. Then name the row and the one thing you need. Say which rows you continue with after it. "what are you waiting for , keep testing" (2026-11-18). "there are still lots of failed items, why are we stopping" (2026-11-18).
- **A defect does not stop the run.** Open a ticket, spawn one Ares in a worktree for it, and continue with the next row. Test the row again after the fix lands. After the last fix lands, run the list again from the first row.
- **A background Ares is not a reason to wait.** Continue with rows the fix does not touch. When no such row is left, show the agent's status and the files it changed so far. "what took you so long" (2026-11-20).

**At the end:**
- Restore the test data from `.claude/.Arena/test-data-register.md`. Compare the database with the snapshot from before the run.
- Report one time: the count of ✓ and ✗ rows, each ✗ row with its ticket, and the path of the list file. Open an HTML report with `<kratos-bin> open <file>`. Give a file path, not a published page. "give me the path" (2026-11-19).
- Build a spec page from the run. The page holds the SA items and the rules the user added during the run. It does not copy the SA text. "你還需要把我們補充的規格記錄到上面" (2026-11-20).

## 3. UI mock (you present, the user judges)

1. **Look before you show.** Screenshot desktop, dark theme and phone width; read the three images yourself; fix what you see. "are you sure you like that toolbar design?" caught an unchecked mock (2026-10-23 11:44); a dark toolbar shipped into the light theme from an unchecked mock (10-24 07:23).
2. **Open it on screen.** `<kratos-bin> open <file>` / the browser window — a file path or a screenshot path in chat is "you did not show me shit" (10-24 08:33).
3. **Cold-open test.** Before presenting, answer in one paragraph: a first-time user opens this page — what do they see, what do they do first, where is the setup, how do they redo it. If you cannot, the mock is not ready ("我真的不知道使用者要怎麼使用", 10-26 13:24).
4. **One place per job.** A job editable on two pages, or two cards for one credential, is a design defect, not a feature ("你只需要一個", 10-26 09:43).
5. **Design proposals** with several options are one HTML page with clickable mock screens, not chat text.

## 4. Demo guide, tutorial, PDF

1. **Inventory first.** Enumerate the feature's screens from code — routes, wizards, bulk toolbars, settings — before taking a single screenshot. A guide built from the bug-fix screenshots at hand was redone three times (2026-10-31 02:23 → 04:22: "看起來不像是從最一開始開始講解", "你沒有介紹批次對應").
2. **Start from an empty tenant** and walk the inventory in the order a new user meets it; every screen gets its one-paragraph "what you see, what you do".
3. **One scoping question** when the source images are test artifacts or the audience is unstated: "first-run guide for a new user, or a feature tour for an existing one?" — the one question that would have saved two rounds.
4. **Screenshots in bulk go to a subagent** that returns a verdict list (`file · what it shows · usable yes/no`), or a contact sheet; reading 60 full-size images inline forced a mid-task compaction (10-31 03:44).
5. Restore a minimized browser window before headed screenshots (30 s vs <1 s, and short UI states are missed).

## 5. Environment facts survive compaction

Whatever you stand up — fake servers and their ports, worktrees and their branches, the demo backend command, the restart script — goes into `.claude/.Arena/handoff.md` under `## Environment` the moment it exists (one line each). The compact SessionStart hook re-injects that section; without it the fake Mallbic server was forgotten after a compaction and the user was asked for a real token ("就跟你說要用 dummy server 了", 10-26 10:15).

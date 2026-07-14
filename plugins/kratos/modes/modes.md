---
description: Execution mode reference — eco/normal/power model routing for all Kratos agents
---

# Kratos: Execution Modes

One reference for all model routing. **Normal** is the default; eco and power are keyword-activated overrides.

## Trigger Keywords

| Mode | Keywords | Motto |
|------|----------|-------|
| **Eco** | `eco`, `ecomode`, `eco-mode`, `efficient`, `save-tokens`, `budget`, `cheap`, `low-cost` | "The cheapest path to victory is the best path." |
| **Power** | `power`, `powermode`, `power-mode`, `max`, `maximum`, `full-power`, `best quality`, `cost no concern` | "Every god fights at full strength." |

Examples: `eco fix the login bug` · `power: review this critical security code`

## Model Matrix

<!-- gen:model-matrix -->
| Agent | Stage | Normal | Eco | Power |
|-------|-------|--------|-----|-------|
| **Metis** | 0 | sonnet | haiku | opus |
| **Athena** | 1 | opus | sonnet | opus |
| **Nemesis** | 2 | opus | sonnet | opus |
| **Daedalus** | 2→3 | sonnet | haiku | opus |
| **Hephaestus** | 4 | opus | sonnet | opus |
| **Apollo** | 5 | opus | haiku | opus |
| **Artemis** | 6 | sonnet | haiku | opus |
| **Ares** | 7a | sonnet | haiku | opus |
| **Hera** | 8 | sonnet | haiku | opus |
| **Cassandra** | 9 | sonnet | haiku | opus |
| **Hermes** | 9 | opus | haiku | opus |
| **Ananke** |  | haiku | haiku | sonnet |
| **Clio** |  | sonnet | haiku | opus |
| **Hades** |  | sonnet | haiku | opus |
| **Iris** |  | sonnet | haiku | opus |
| **Mimir** |  | sonnet | haiku | opus |
| **Odysseus** |  | sonnet | haiku | opus |
| **Prometheus** |  | opus | sonnet | opus |
| **Themis** |  | sonnet | haiku | opus |
<!-- /gen:model-matrix -->

Quick-mode tasks (tests, fixes, refactor, review, research, docs, debug, plan): **normal = sonnet, eco = haiku, power = opus**.

<!-- gen:model-matrix-summary -->
Summary: Normal ≈ 6 Opus / 12 Sonnet / 1 Haiku · Eco ≈ 0 Opus / 4 Sonnet / 15 Haiku (~70-85% cheaper) · Power ≈ 18 Opus / 1 Sonnet / 0 Haiku (~2-7× cost).
<!-- /gen:model-matrix-summary -->

## How to Spawn

Set the `model` param from the matrix and prefix the prompt in eco/power:

```
Task(
  subagent_type: "kratos:[agent]",
  model: "[model from matrix]",
  prompt: "[ECO MODE — be concise, minimize verbose output. | POWER MODE — maximum quality, thorough analysis.]

MISSION: [task]
[mission details]",
  description: "[agent]-[mode] - [task]"
)
```

Announce before spawning (`[MODE]: [request] → [agent] ([model])`) and report completion (`[MODE] COMPLETE: [agent] ([model]) — [summary]`).

## Choosing a Mode

**Power is worth it for**: security-critical features (auth, payments, encryption), complex architecture, production-critical validation, deep debugging, high-stakes reviews. Avoid it for simple fixes/docs/tests where Opus adds nothing.

**Eco is risky for**: security-critical review, complex architectural decisions, auth/encryption changes, financial logic, data migration, public API changes. If the user requests eco for one of these, confirm first:

```
AskUserQuestion(
  question: "ECO WARNING: This task benefits from higher-tier models because [reason]. Continue with eco mode anyway?",
  options: ["Yes, proceed with eco", "No, use normal mode", "No, use power mode", "Let me type it"]
)
```

## Rules

1. Use the matrix model for every spawn — never guess.
2. Stay in the chosen mode until the user says otherwise.
3. Eco: report estimated savings; warn on risky tasks. Power: no cost warnings — the user chose it.
4. Still delegate — never do the work yourself.

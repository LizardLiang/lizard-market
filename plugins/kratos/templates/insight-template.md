# Mimir Insight — Cached Research File

Fetched by Mimir with `<kratos-bin> template get insight-template`. Path: `.claude/.Arena/insights/<topic-slug>-<YYYY-MM-DD>.md` — slug lowercase, hyphenated, topic + technology, under 50 chars (`rate-limiting-nodejs-2026-02-06.md`, `cve-react-dom-2026-02-06.md`).

```markdown
# [Topic] Research

## Metadata
| Field | Value |
|-------|-------|
| **Researched** | YYYY-MM-DD |
| **TTL** | N days |
| **Query** | [Original question asked] |
| **Researcher** | Mimir |
| **Cache Until** | YYYY-MM-DD (UTC) |

## Summary
[2-3 paragraph executive summary of findings]

## Key Findings

### Approach 1: [Name]
**Source**: [GitHub repo or doc link]
**Pros**: [list]
**Cons**: [list]
**Example**:
```[language]
[Code example if applicable]
```

### Approach 2: [Name]
[Same structure]

## Recommendations
Based on this project's context ([relevant tech stack]):
1. **[Recommendation 1]** - [Reasoning]
2. **[Recommendation 2]** - [Reasoning]

## Sources Consulted
- [URL 1] - [Description]
- [GitHub repo] - [stars]

## Related Topics
- [Related topic] - For further research
```

## Mission recipes (all: clean stale insights → gather → analyze → cache or return)

| Mission | Gather | Cache TTL |
|---------|--------|-----------|
| GitHub best practices | `gh search repos "<topic>" --sort stars --limit 10 --json name,owner,stars,description`; `gh search code "<pattern>" --language <lang> --limit 5`; WebFetch READMEs of 3–5 top repos; compare 2–3 approaches | 30 days |
| API documentation | WebFetch official docs; `gh search code "<library> example" --limit 5`; `npm view <package>`; document auth, rate limits, key endpoints, version compatibility | 14 days |
| Security advisory | WebFetch `https://cve.mitre.org/cgi-bin/cvekey.cgi?keyword=<package>`; `gh api "/advisories?severity=high&ecosystem=npm"`; `npm audit --json`; summarize CVEs, severity, affected versions, remediation | 7 days |
| Documentation lookup | Official site / README / npm docs; extract usage, config options, pitfalls | 14 days if the library is in the project manifest, else return directly |
| Stack Overflow | `https://stackoverflow.com/search?q=<query>`; top answers with votes/acceptance; most common solution + gotchas | no cache |

# Mimir - Your Research Oracle

Summon **Mimir** before major PRD work to gather knowledge from the outside world. Mimir covers broad research (competitors, best practices, examples); context7 covers precise API specifications. Use both together for the most complete picture.

### When to Summon Mimir

- Before creating a PRD — research competitors, market trends, best practices
- When the feature involves external APIs — gather comprehensive API documentation
- When requirements are unclear — research domain knowledge and industry standards
- For security-sensitive features — research security best practices and known vulnerabilities

### How to Summon Mimir

```
Task(
  subagent_type: "kratos:mimir",
  model: "sonnet",
  prompt: "MISSION: External Research for PRD
TOPIC: [what to research]
FOCUS: [specific questions to answer]
FEATURE: [feature name for context]

Research using web, GitHub, documentation sites, and Notion (if applicable). Your findings will be used by Athena for the PRD.

If findings are broadly useful (best practices, architectural patterns), cache to .claude/.Arena/insights/ with appropriate TTL.

Return comprehensive but concise summary.",
  description: "mimir - research for [topic]"
)
```

### Research Integration Workflow

```
1. Athena identifies knowledge gap during PRD creation
2. Athena spawns Mimir with specific research questions
3. Mimir researches: GitHub repos, official docs, best practices, security, Notion
4. Mimir returns findings and optionally caches insights
5. Athena incorporates findings into PRD
6. Athena credits Mimir in the "External Research Summary" section
```

### Mimir vs Context7

| Tool         | Use When                                                    | Output                                |
| ------------ | ----------------------------------------------------------- | ------------------------------------- |
| **Mimir**    | Research approaches, best practices, broad understanding    | Research summary with recommendations |
| **context7** | Need exact API method signatures, library version specifics | Precise API specifications            |

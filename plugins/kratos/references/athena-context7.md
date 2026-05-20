# Context7 - API Specification Gathering

When the feature involves external APIs or libraries, use context7 to get accurate, up-to-date specifications — your training data may not have the latest method signatures or breaking changes.

### How to Use Context7

Use the context7 MCP tools directly (they are available in your tool list):

1. Resolve library ID:

```
mcp__plugin_context7_context7__resolve-library-id(libraryName: "stripe")
```

1. Get documentation:

```
mcp__plugin_context7_context7__query-docs(
  context7CompatibleLibraryID: "/stripe/stripe-node",
  topic: "payment intents"
)
```

**Note:** If context7 tools are unavailable in your environment, delegate to Mimir for API research instead.

### Document API Findings

Add an **External APIs** section to your PRD:

```markdown
## 8. External API Dependencies

### [API Name]

| Aspect             | Details                 |
| ------------------ | ----------------------- |
| **Library**        | [library name]          |
| **Version**        | [version from context7] |
| **Key Endpoints**  | [relevant endpoints]    |
| **Authentication** | [auth method]           |
| **Rate Limits**    | [if applicable]         |
```

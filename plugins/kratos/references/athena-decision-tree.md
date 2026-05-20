# Decision Tree Format

Maintain the decision tree as a live ASCII diagram throughout the GAP_ANALYSIS conversation. Update it after every answer.

**Format rules:**
- Root line: `Feature: <name>`
- Each gap is a branch: `├──` (mid-list) or `└──` (last item)
- Branch text: `<gap label>? → <answer or status>`
- Sub-questions revealed by an answer are indented under the parent using `│   ├──` / `│   └──`
- Status markers:
  - `✓` — resolved (answer recorded)
  - `[open]` — identified but not yet asked
  - `[leaf]` — resolved with no further sub-questions
  - `[assumed: X]` — gap documented as assumption, not asked

**Example (mid-conversation):**
```
Feature: File Upload
├── Storage backend? → S3 ✓
│   ├── Size limit? → 25MB ✓
│   └── CDN? → CloudFront ✓ [leaf]
├── File types? → [open]
│   └── Validation? → [depends on file types]
└── Auth required? → [open]
```

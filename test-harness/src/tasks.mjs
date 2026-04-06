/**
 * Kratos test task definitions.
 *
 * Each task exercises a different Kratos routing path:
 *   - implementation   → /kratos:main → full 11-stage pipeline
 *   - debug            → /kratos:quick → Hades agent
 *   - research-metis   → /kratos:inquiry → Metis agent (project info)
 *   - research-mimir   → /kratos:inquiry → Mimir agent (external research)
 *   - research-clio    → /kratos:inquiry → Clio agent (git history)
 *   - brainstorming    → /kratos:plan → Prometheus
 *
 * Edit prompts here to customise what gets tested.
 */

export const TASKS = [
  {
    name: "implementation",
    type: "full-pipeline",
    description: "Triggers the full 11-stage Kratos pipeline via /kratos:main",
    prompt:
      "/kratos:main build a CLI utility in Go named `cfg-summary` that reads a flat key-value JSON config file (e.g. {\"app\": \"myapp\", \"version\": \"1.0\", \"port\": 8080}) and prints a two-column ASCII table (Key | Value) to stdout. Requirements: --config <path> flag (required), --output flag accepting \"plain\" (default) or \"json\" (prints JSON array of {key, value} objects). On missing file or invalid JSON, print error to stderr and exit code 1. No external dependencies — stdlib only. Target: Linux/macOS/Windows.",
  },
  {
    name: "debug",
    type: "quick-hades",
    description: "Routes to Hades agent for root-cause debugging",
    prompt:
      "/kratos:quick debug why calling `json.Unmarshal` into a map[string]interface{} sometimes silently drops nested null values instead of preserving them as nil.",
  },
  {
    name: "research-metis",
    type: "inquiry-metis",
    description: "Routes to Metis for project/code exploration",
    prompt:
      "/kratos:inquiry how does the Kratos pipeline stage progression work? Walk me through how status.json is updated as a feature moves from PRD through to code review.",
  },
  {
    name: "research-mimir",
    type: "inquiry-mimir",
    description: "Routes to Mimir for external research and best practices",
    prompt:
      "/kratos:inquiry what are the best practices for implementing multi-agent AI systems? Find examples on GitHub and show me popular patterns for agent orchestration.",
  },
  {
    name: "research-clio",
    type: "inquiry-clio",
    description: "Routes to Clio for git history analysis",
    prompt:
      "/kratos:inquiry who has been the most active contributor to this project in the last 6 months? Show me the recent commit history and blame information.",
  },
  {
    name: "brainstorming",
    type: "plan-prometheus",
    description: "Routes to Prometheus for strategic planning and brainstorming",
    prompt:
      "/kratos:plan a plugin marketplace search feature that lets users discover and install Claude Code plugins from a central registry via CLI.",
  },
  // CLI compliance tests — verify each pipeline agent uses CLI (not direct JSON writes) for status.json
  // These run sequentially against the shared _test-project so each stage builds on the previous.
  {
    name: "cli-compliance-athena",
    type: "cli-compliance-stage-1",
    description: "Tests Athena (Stage 1) uses CLI to init and update status.json",
    prompt:
      "/kratos:main build a minimal HTTP health-check endpoint in Go. Feature name: cli-compliance-test. This is a CLI compliance test — ensure all status.json updates go through the kratos CLI.",
  },
  {
    name: "cli-compliance-nemesis",
    type: "cli-compliance-stage-2",
    description: "Tests Nemesis (Stage 2) uses CLI to record verdict in status.json",
    prompt:
      "/kratos:main the cli-compliance-test feature needs its PRD reviewed. Advance to the next stage.",
  },
  {
    name: "cli-compliance-hephaestus",
    type: "cli-compliance-stage-5",
    description: "Tests Hephaestus (Stage 5) uses CLI to update status.json with tech spec",
    prompt:
      "/kratos:main create the technical specification for cli-compliance-test. Advance through any pending stages.",
  },
  {
    name: "cli-compliance-apollo",
    type: "cli-compliance-stage-6",
    description: "Tests Apollo (Stage 6) uses CLI to record spec review in status.json",
    prompt:
      "/kratos:main review the technical spec for cli-compliance-test. Advance to the next stage.",
  },
  {
    name: "cli-compliance-artemis",
    type: "cli-compliance-stage-8",
    description: "Tests Artemis (Stage 8) uses CLI to update status.json with test plan",
    prompt:
      "/kratos:main create the test plan for cli-compliance-test. Advance through any pending stages.",
  },
  {
    name: "cli-compliance-ares",
    type: "cli-compliance-stage-9",
    description: "Tests Ares (Stage 9) uses CLI to update status.json during implementation",
    prompt:
      "/kratos:main implement the cli-compliance-test feature. Use Ares mode (AI implementation).",
  },
  {
    name: "cli-compliance-hera",
    type: "cli-compliance-stage-10",
    description: "Tests Hera (Stage 10) uses CLI to record PRD alignment verdict in status.json",
    prompt:
      "/kratos:main run the PRD alignment check for cli-compliance-test. Advance to the next stage.",
  },
  {
    name: "cli-compliance-hermes",
    type: "cli-compliance-stage-11",
    description: "Tests Hermes (Stage 11) uses CLI to record code review verdict in status.json",
    prompt:
      "/kratos:main run the code review for cli-compliance-test. Advance to the next stage.",
  },
  {
    name: "cli-compliance-cassandra",
    type: "cli-compliance-stage-11b",
    description: "Tests Cassandra (Stage 11) uses CLI to record risk analysis in status.json",
    prompt:
      "/kratos:main run the risk analysis audit for cli-compliance-test.",
  },
  // Timestamp validation tests for status.json updating agents
  {
    name: "timestamp-athena",
    type: "stage-1-athena",
    description: "Tests Athena (Stage 1) timestamp updates in status.json",
    prompt:
      "/kratos:main create a PRD for a simple note-taking CLI app. The task name should be timestamp-test-athena. Focus on ensuring proper timestamp updates.",
  },
  {
    name: "timestamp-nemesis",
    type: "stage-2-nemesis",
    description: "Tests Nemesis (Stage 2) timestamp updates in status.json",
    prompt:
      "/kratos:main review the PRD for timestamp-test-athena feature. Ensure you update timestamps correctly when marking your review complete.",
  },
  {
    name: "timestamp-hephaestus",
    type: "stage-3-hephaestus",
    description: "Tests Hephaestus (Stage 3) timestamp updates in status.json",
    prompt:
      "/kratos:main create a technical specification for timestamp-test-athena feature. Verify timestamps are properly set during status updates.",
  },
  {
    name: "timestamp-apollo",
    type: "stage-4-apollo",
    description: "Tests Apollo (Stage 4) timestamp updates in status.json",
    prompt:
      "/kratos:main review the technical specification for timestamp-test-athena. Ensure proper timestamp handling in your review process.",
  },
  {
    name: "timestamp-artemis",
    type: "stage-6-artemis",
    description: "Tests Artemis (Stage 6) timestamp updates in status.json",
    prompt:
      "/kratos:main create a test plan for timestamp-test-athena feature. Validate that your status.json updates use correct timestamps.",
  },
  {
    name: "timestamp-ares",
    type: "stage-7-ares",
    description: "Tests Ares (Stage 7) timestamp updates in status.json",
    prompt:
      "/kratos:main implement the timestamp-test-athena feature. Focus on proper timestamp tracking during implementation stages.",
  },
  {
    name: "timestamp-hermes",
    type: "stage-8-hermes",
    description: "Tests Hermes (Stage 8) timestamp updates in status.json",
    prompt:
      "/kratos:main review the code for timestamp-test-athena feature. Ensure your review process maintains correct timestamp integrity.",
  },
  // Context optimization comparison tests (same task, different pipeline routes)
  {
    name: "compare-original",
    type: "full-pipeline-original",
    description: "Baseline: standard /kratos:main pipeline",
    prompt:
      "/kratos:main build a key-value store CLI in Go named `kv` with set/get/delete/list commands backed by a local JSON file. Support --file flag to specify storage path. Stdlib only.",
  },
  {
    name: "compare-optimized",
    type: "full-pipeline-optimized",
    description: "Optimized: /kratos:test-optimized pipeline with context curation",
    prompt:
      "/kratos:test-optimized build a key-value store CLI in Go named `kv` with set/get/delete/list commands backed by a local JSON file. Support --file flag to specify storage path. Stdlib only.",
  },
];

/**
 * Return a subset of tasks by name, or all tasks if names is empty.
 * @param {string[]} names
 * @returns {typeof TASKS}
 */
export function selectTasks(names) {
  if (!names || names.length === 0) return TASKS;
  return TASKS.filter((t) => names.includes(t.name));
}

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
      "Kratos, build a CLI utility in Go that reads a JSON config file and prints a formatted summary table to stdout. It should support --config flag and --output flag for plain or json formats.",
  },
  {
    name: "debug",
    type: "quick-hades",
    description: "Routes to Hades agent for root-cause debugging",
    prompt:
      "Kratos, debug why calling `json.Unmarshal` into a map[string]interface{} sometimes silently drops nested null values instead of preserving them as nil.",
  },
  {
    name: "research-metis",
    type: "inquiry-metis",
    description: "Routes to Metis for project/code exploration",
    prompt:
      "Kratos, how does the Kratos pipeline stage progression work? Walk me through how status.json is updated as a feature moves from PRD through to code review.",
  },
  {
    name: "research-mimir",
    type: "inquiry-mimir",
    description: "Routes to Mimir for external research and best practices",
    prompt:
      "Kratos, what are the best practices for implementing multi-agent AI systems? Find examples on GitHub and show me popular patterns for agent orchestration.",
  },
  {
    name: "research-clio",
    type: "inquiry-clio",
    description: "Routes to Clio for git history analysis",
    prompt:
      "Kratos, who has been the most active contributor to this project in the last 6 months? Show me the recent commit history and blame information.",
  },
  {
    name: "brainstorming",
    type: "plan-prometheus",
    description: "Routes to Prometheus for strategic planning and brainstorming",
    prompt:
      "Kratos, plan a plugin marketplace search feature that lets users discover and install Claude Code plugins from a central registry via CLI.",
  },
  // Timestamp validation tests for status.json updating agents
  {
    name: "timestamp-athena",
    type: "stage-1-athena",
    description: "Tests Athena (Stage 1) timestamp updates in status.json",
    prompt:
      "Kratos, create a PRD for a simple note-taking CLI app. The task name should be timestamp-test-athena. Focus on ensuring proper timestamp updates.",
  },
  {
    name: "timestamp-nemesis",
    type: "stage-2-nemesis",
    description: "Tests Nemesis (Stage 2) timestamp updates in status.json",
    prompt:
      "Kratos, review the PRD for timestamp-test-athena feature. Ensure you update timestamps correctly when marking your review complete.",
  },
  {
    name: "timestamp-hephaestus",
    type: "stage-3-hephaestus",
    description: "Tests Hephaestus (Stage 3) timestamp updates in status.json",
    prompt:
      "Kratos, create a technical specification for timestamp-test-athena feature. Verify timestamps are properly set during status updates.",
  },
  {
    name: "timestamp-apollo",
    type: "stage-4-apollo",
    description: "Tests Apollo (Stage 4) timestamp updates in status.json",
    prompt:
      "Kratos, review the technical specification for timestamp-test-athena. Ensure proper timestamp handling in your review process.",
  },
  {
    name: "timestamp-artemis",
    type: "stage-6-artemis",
    description: "Tests Artemis (Stage 6) timestamp updates in status.json",
    prompt:
      "Kratos, create a test plan for timestamp-test-athena feature. Validate that your status.json updates use correct timestamps.",
  },
  {
    name: "timestamp-ares",
    type: "stage-7-ares",
    description: "Tests Ares (Stage 7) timestamp updates in status.json",
    prompt:
      "Kratos, implement the timestamp-test-athena feature. Focus on proper timestamp tracking during implementation stages.",
  },
  {
    name: "timestamp-hermes",
    type: "stage-8-hermes",
    description: "Tests Hermes (Stage 8) timestamp updates in status.json",
    prompt:
      "Kratos, review the code for timestamp-test-athena feature. Ensure your review process maintains correct timestamp integrity.",
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

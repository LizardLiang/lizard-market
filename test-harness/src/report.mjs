import fs from "fs";
import path from "path";

/**
 * Generate report.json from per-task summary.json files.
 * Call after all tasks have finished.
 *
 * @param {string} runDir     - path to the run directory (e.g. results/<run-id>/)
 * @param {string[]} taskNames - ordered list of task names that were run
 * @param {object} runMeta    - the run-meta.json object
 * @returns {object} the report object (also written to runDir/report.json)
 */
export function generateReport(runDir, taskNames, runMeta) {
  const taskSummaries = [];

  for (const name of taskNames) {
    const summaryPath = path.join(runDir, name, "summary.json");
    if (!fs.existsSync(summaryPath)) continue;
    const summary = JSON.parse(fs.readFileSync(summaryPath, "utf8"));
    taskSummaries.push(summary);
  }

  const successCount = taskSummaries.filter((s) => s.status === "success").length;
  const errorCount = taskSummaries.filter((s) => s.status === "error").length;
  const totalTokens = taskSummaries.reduce((sum, s) => sum + (s.tokens?.total ?? 0), 0);
  const totalDurationMs = taskSummaries.reduce((sum, s) => sum + (s.durationMs ?? 0), 0);

  // Collect all unique agents seen across all tasks
  const allAgents = new Set();
  for (const s of taskSummaries) {
    for (const a of s.agentsSpawned ?? []) allAgents.add(a);
  }

  // Build comparison table rows
  const comparison = taskSummaries.map((s) => ({
    task: s.task,
    status: s.status,
    durationSec: s.durationSec,
    totalMessages: s.messageCounts?.total ?? 0,
    toolCalls: s.messageCounts?.tool_use ?? 0,
    thinkingBlocks: s.messageCounts?.thinking ?? 0,
    agentsSpawned: s.agentCount ?? 0,
    agentNames: s.agentsSpawned ?? [],
    inputTokens: s.tokens?.input ?? 0,
    outputTokens: s.tokens?.output ?? 0,
    errors: s.errors?.length ?? 0,
  }));

  const report = {
    runId: runMeta.runId,
    kratosPuginPath: runMeta.kratosPluginPath,
    model: runMeta.model,
    startedAt: runMeta.startedAt,
    finishedAt: new Date().toISOString(),
    totalDurationSec: +(totalDurationMs / 1000).toFixed(1),
    tasksRun: taskSummaries.length,
    tasksSucceeded: successCount,
    tasksFailed: errorCount,
    totalTokens,
    allAgentsSeen: [...allAgents].sort(),
    comparison,
    health: deriveHealth(taskSummaries),
  };

  fs.writeFileSync(
    path.join(runDir, "report.json"),
    JSON.stringify(report, null, 2)
  );

  return report;
}

/**
 * Derive high-level health indicators from all task summaries.
 */
function deriveHealth(summaries) {
  const total = summaries.length;
  if (total === 0) return { rating: "unknown", notes: [] };

  const successRate = summaries.filter((s) => s.status === "success").length / total;
  const avgDurationSec =
    summaries.reduce((s, t) => s + (t.durationSec ?? 0), 0) / total;
  const totalErrors = summaries.reduce((s, t) => s + (t.errors?.length ?? 0), 0);

  const notes = [];

  if (successRate < 1) notes.push(`${Math.round((1 - successRate) * 100)}% of tasks failed`);
  if (avgDurationSec > 300) notes.push(`Average task duration ${avgDurationSec.toFixed(0)}s is high`);
  if (totalErrors > 0) notes.push(`${totalErrors} total error(s) captured`);

  // Check if Kratos routed to expected agents
  const implTask = summaries.find((s) => s.task === "implementation");
  const debugTask = summaries.find((s) => s.task === "debug");
  const researchMetisTask = summaries.find((s) => s.task === "research-metis");
  const researchMimirTask = summaries.find((s) => s.task === "research-mimir");
  const researchClioTask = summaries.find((s) => s.task === "research-clio");
  const brainTask = summaries.find((s) => s.task === "brainstorming");

  if (implTask?.status === "success" && implTask.agentCount === 0) {
    notes.push("implementation task completed but spawned no agents — pipeline may not have triggered");
  }
  if (debugTask?.status === "success" && !debugTask.agentsSpawned?.some((a) => a.includes("hades"))) {
    notes.push("debug task did not spawn Hades — routing may be incorrect");
  }
  if (researchMetisTask?.status === "success" && !researchMetisTask.agentsSpawned?.some((a) =>
    a.includes("metis") || a.includes("general-purpose")
  )) {
    notes.push("research-metis task did not spawn Metis/general-purpose — routing may be incorrect");
  }
  if (researchMimirTask?.status === "success" && !researchMimirTask.agentsSpawned?.some((a) => a.includes("mimir"))) {
    notes.push("research-mimir task did not spawn Mimir — routing may be incorrect");
  }
  if (researchClioTask?.status === "success" && !researchClioTask.agentsSpawned?.some((a) => a.includes("clio"))) {
    notes.push("research-clio task did not spawn Clio — routing may be incorrect");
  }
  if (brainTask?.status === "success" && !brainTask.agentsSpawned?.some((a) => a.includes("prometheus"))) {
    notes.push("brainstorming task did not spawn Prometheus — routing may be incorrect");
  }

  const rating =
    successRate === 1 && totalErrors === 0 && notes.length === 0
      ? "green"
      : successRate >= 0.75
      ? "yellow"
      : "red";

  return { rating, successRate: +successRate.toFixed(2), avgDurationSec: +avgDurationSec.toFixed(1), notes };
}

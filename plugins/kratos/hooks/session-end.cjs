#!/usr/bin/env node
/**
 * Kratos Memory - Session End Hook
 *
 * Automatically ends a memory session when Claude Code session ends.
 * Generates a summary and saves it to the global database.
 * 
 * NEW: Detects which feature was being worked on from .claude/feature/[name]/status.json
 */

const { execSync } = require('child_process');
const path = require('path');
const fs = require('fs');
const os = require('os');
const { resolveBinary } = require('./kratos-bin.cjs');

// Global paths
const KRATOS_HOME = path.join(os.homedir(), '.kratos');
const DB_PATH = path.join(KRATOS_HOME, 'memory.db');
const SESSION_FILE = path.join(KRATOS_HOME, 'active-session.json');

// Get current working directory
const cwd = process.cwd();

const findKratosBinary = resolveBinary;

// Read session data
function getSession() {
  if (!fs.existsSync(SESSION_FILE)) return null;
  try {
    return JSON.parse(fs.readFileSync(SESSION_FILE, 'utf-8'));
  } catch (e) {
    return null;
  }
}

// Find active feature from .claude/feature/*/status.json
function findActiveFeature() {
  const featureDir = path.join(cwd, '.claude', 'feature');
  
  if (!fs.existsSync(featureDir)) return null;
  
  try {
    const features = fs.readdirSync(featureDir);
    let mostRecent = null;
    let mostRecentTime = 0;
    
    for (const featureName of features) {
      const statusPath = path.join(featureDir, featureName, 'status.json');
      
      if (fs.existsSync(statusPath)) {
        try {
          const stats = fs.statSync(statusPath);
          const statusData = JSON.parse(fs.readFileSync(statusPath, 'utf-8'));
          
          // Check if this is an in-progress feature
          const featureStatus = statusData.status || statusData.feature?.status;
          if (featureStatus !== 'completed' && featureStatus !== 'abandoned') {
            // Track most recently modified
            if (stats.mtimeMs > mostRecentTime) {
              mostRecentTime = stats.mtimeMs;
              mostRecent = {
                name: featureName,
                stage: statusData.current_stage || statusData.stage || 0,
                status: featureStatus || 'in_progress',
                statusPath: statusPath
              };
            }
          }
        } catch (e) {
          // Skip invalid status files
        }
      }
    }
    
    return mostRecent;
  } catch (e) {
    return null;
  }
}

// Get session statistics
function getSessionStats(sessionId) {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return { totalSteps: 0, agentSpawns: 0, fileChanges: 0 };

  try {
    const result = execSync(
      `"${kratosCmd}" step list "${sessionId}"`,
      {
        encoding: 'utf-8',
        env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH }
      }
    );
    const data = JSON.parse(result);
    const steps = data.steps || [];

    const agentSpawns = steps.filter(s => s.step_type === 'agent_spawn').length;
    const fileChanges = steps.filter(s => s.step_type === 'file_modify').length;

    return {
      totalSteps: steps.length,
      agentSpawns,
      fileChanges
    };
  } catch (e) {
    return { totalSteps: 0, agentSpawns: 0, fileChanges: 0 };
  }
}

// End session
function endSession(sessionId, summary, status = 'completed', featureName = null) {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return false;

  try {
    execSync(
      `"${kratosCmd}" session end "${sessionId}" "${escapeShell(summary)}"`,
      {
        stdio: 'ignore',
        env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH }
      }
    );
    return true;
  } catch (e) {
    return false;
  }
}

// Escape shell characters
function escapeShell(str) {
  if (!str) return '';
  return str.replace(/"/g, '\\"').replace(/\n/g, ' ').substring(0, 500);
}

// Format duration
function formatDuration(ms) {
  const minutes = Math.floor(ms / 60000);
  if (minutes < 60) return `${minutes} minutes`;
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  return `${hours}h ${mins}m`;
}

// Get un-archived spec deltas across all features (spec-lifecycle durability reminder).
// Returns a list of display lines, or [] if none or the binary is unavailable.
function getPendingSpecDeltas() {
  const kratosCmd = findKratosBinary();
  if (!kratosCmd) return [];

  try {
    const result = execSync(`"${kratosCmd}" spec list --changes`, { encoding: 'utf-8' });
    const lines = result.split('\n').map(l => l.trim()).filter(Boolean);
    if (lines.length === 0) return [];
    if (lines.length === 1 && /no pending spec deltas/i.test(lines[0])) return [];
    return lines;
  } catch (e) {
    return [];
  }
}

// Report pending spec deltas regardless of whether a memory session is active —
// this is the "never lost" reminder for deltas that survived past a skipped Hera
// stage, an abandoned feature, or User Mode.
function reportPendingSpecDeltas() {
  const pending = getPendingSpecDeltas();
  if (pending.length === 0) return;

  console.log(`Kratos: ${pending.length} un-archived spec delta(s) pending`);
  for (const line of pending) {
    console.log(`  ${line}`);
  }
  console.log('  Promote with: /kratos:spec-archive <feature>  or  kratos spec archive <feature>');
}

// Find tactical plans still marked `status: draft` — plan sessions that ended
// before their clarification loop reached PLAN_READY. Their Locked Decisions are
// real user answers, so the session must not end silently on top of them.
// Pure fs, no binary dependency (unlike the spec-delta reporter above).
function getDraftPlans() {
  const planDir = path.join(cwd, '.claude', '.Arena', 'tactical-plans');
  if (!fs.existsSync(planDir)) return [];

  const drafts = [];
  try {
    for (const name of fs.readdirSync(planDir)) {
      if (!name.endsWith('.md')) continue;
      const full = path.join(planDir, name);
      try {
        // Frontmatter lives in the first few lines; read the head only.
        const head = fs.readFileSync(full, 'utf-8').slice(0, 512);
        if (!/^---\r?\n(?:.*\r?\n)*?status:\s*draft\b/m.test(head)) continue;
        const decisions = countLockedDecisions(full);
        drafts.push({ name, decisions });
      } catch (e) {
        // Unreadable file — skip it rather than failing the whole hook.
      }
    }
  } catch (e) {
    return [];
  }
  return drafts;
}

// Count entries under the plan's `## Locked Decisions` heading, so the reminder
// can say how much answered-question work is sitting in the draft.
function countLockedDecisions(filePath) {
  try {
    const body = fs.readFileSync(filePath, 'utf-8');
    const section = body.split(/^##\s+Locked Decisions\s*$/m)[1];
    if (!section) return 0;
    const untilNextHeading = section.split(/^##\s+/m)[0];
    return (untilNextHeading.match(/^\s*-\s+\*\*/gm) || []).length;
  } catch (e) {
    return 0;
  }
}

// Report unfinished plan drafts, on the same "never lost" principle as pending
// spec deltas — these hold decisions the user cannot regenerate.
function reportDraftPlans() {
  const drafts = getDraftPlans();
  if (drafts.length === 0) return;

  console.log(`Kratos: ${drafts.length} unfinished plan draft(s) in .claude/.Arena/tactical-plans/`);
  for (const d of drafts) {
    const count = d.decisions === 1 ? '1 locked decision' : `${d.decisions} locked decisions`;
    console.log(`  ${d.name} (${count})`);
  }
  console.log('  Resume with: /kratos:plan <task>  — it picks the draft back up instead of re-asking');
}

// Main
function main() {
  const session = getSession();
  if (!session) {
    console.log('Kratos: No active session to end');
    reportPendingSpecDeltas();
    reportDraftPlans();
    return;
  }

  const { session_id, project, started_at } = session;
  const duration = formatDuration(Date.now() - started_at);

  // Detect active feature
  const activeFeature = findActiveFeature();

  // Get stats
  const stats = getSessionStats(session_id);

  // Generate summary
  let summary = `Session in ${project} (${duration}): ${stats.totalSteps} steps, ${stats.agentSpawns} agents spawned`;
  if (activeFeature) {
    summary += `. Feature: ${activeFeature.name} (stage ${activeFeature.stage})`;
  }

  // End the session
  if (endSession(session_id, summary)) {
    // Remove session file
    try {
      fs.unlinkSync(SESSION_FILE);
    } catch (e) {
      // Ignore
    }
    console.log(`Kratos: Session ended - ${session_id}`);
    console.log(`  Duration: ${duration}`);
    console.log(`  Steps: ${stats.totalSteps}`);
    console.log(`  Agents: ${stats.agentSpawns}`);
    if (activeFeature) {
      console.log(`  Feature: ${activeFeature.name} (stage ${activeFeature.stage})`);
    }
  }

  reportPendingSpecDeltas();
  reportDraftPlans();
}

main();

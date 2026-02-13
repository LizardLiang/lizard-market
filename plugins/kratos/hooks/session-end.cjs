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

// Global paths
const KRATOS_HOME = path.join(os.homedir(), '.kratos');
const DB_PATH = path.join(KRATOS_HOME, 'memory.db');
const SESSION_FILE = path.join(KRATOS_HOME, 'active-session.json');

// Get current working directory
const cwd = process.cwd();

// Find kratos binary
function findKratosBinary() {
  const locations = [
    'kratos', // In PATH
    path.join(__dirname, '..', 'bin', 'kratos'), // Local bin
    path.join(__dirname, '..', 'bin', 'kratos.exe'), // Windows local bin
    path.join(os.homedir(), 'bin', 'kratos'), // User bin
    path.join(os.homedir(), 'bin', 'kratos.exe'), // Windows user bin
  ];

  for (const loc of locations) {
    try {
      execSync(`"${loc}" --version`, { stdio: 'ignore' });
      return loc;
    } catch (e) {}
  }

  return null;
}

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

// Update session with feature info
// TODO: Implement feature update in Go CLI
function updateSessionFeature(sessionId, featureName, stage) {
  // Feature tracking not yet implemented in Go CLI
  // Skipping for now
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

// Main
function main() {
  const session = getSession();
  if (!session) {
    console.log('Kratos: No active session to end');
    return;
  }

  const { session_id, project, started_at } = session;
  const duration = formatDuration(Date.now() - started_at);

  // Detect active feature
  const activeFeature = findActiveFeature();
  if (activeFeature) {
    updateSessionFeature(session_id, activeFeature.name, activeFeature.stage);
  }

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
}

main();

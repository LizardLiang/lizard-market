#!/usr/bin/env node
/**
 * Kratos Memory - Session End Hook
 *
 * Automatically ends a memory session when Claude Code session ends.
 * Generates a summary and saves it to the global database.
 * 
 * NEW: Detects which feature was being worked on from .claude/feature/*/status.json
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

// Check if Python is available
function getPythonCmd() {
  try {
    const { getPythonCmd: getCmd } = require('./check-python.cjs');
    return getCmd();
  } catch (e) {
    // Fallback: try common commands
    for (const cmd of ['python3', 'python']) {
      try {
        execSync(`${cmd} --version`, { stdio: 'ignore' });
        return cmd;
      } catch (e) {}
    }
    return null;
  }
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
function updateSessionFeature(sessionId, featureName, stage) {
  const pythonCmd = getPythonCmd();
  if (!pythonCmd) return;

  try {
    const pythonScript = path.join(__dirname, '..', 'memory', 'kratos_memory.py');
    
    // Update session record
    execSync(
      `${pythonCmd} "${pythonScript}" feature update "${featureName}" "${path.basename(cwd)}" --stage=${stage}`,
      {
        stdio: 'ignore',
        env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH }
      }
    );
  } catch (e) {
    // Ignore errors
  }
}

// Get session statistics
function getSessionStats(sessionId) {
  const pythonCmd = getPythonCmd();
  if (!pythonCmd) return { totalSteps: 0, agentSpawns: 0, fileChanges: 0 };

  try {
    const pythonScript = path.join(__dirname, '..', 'memory', 'kratos_memory.py');
    const result = execSync(
      `${pythonCmd} "${pythonScript}" query steps "${sessionId}"`,
      {
        encoding: 'utf-8',
        env: { ...process.env, KRATOS_MEMORY_DB: DB_PATH }
      }
    );
    const steps = JSON.parse(result);

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
  const pythonCmd = getPythonCmd();
  if (!pythonCmd) return false;

  try {
    const pythonScript = path.join(__dirname, '..', 'memory', 'kratos_memory.py');
    execSync(
      `${pythonCmd} "${pythonScript}" session end "${sessionId}" "${escapeShell(summary)}" "${status}"`,
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

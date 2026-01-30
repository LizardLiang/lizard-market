#!/usr/bin/env node
/**
 * Kratos Memory - Python Availability Checker
 * 
 * Utility module to check Python availability for the memory system.
 * Used by session hooks and the install script.
 */

const { execSync } = require('child_process');

/**
 * Check if Python is available and return info about it.
 * @returns {{ available: boolean, command: string|null, version: string|null }}
 */
function checkPython() {
  const commands = ['python3', 'python'];
  
  for (const cmd of commands) {
    try {
      const version = execSync(`${cmd} --version`, { 
        encoding: 'utf-8',
        stdio: ['pipe', 'pipe', 'pipe']
      }).trim();
      
      // Verify it's Python 3
      if (version.includes('Python 3')) {
        return {
          available: true,
          command: cmd,
          version: version.replace('Python ', '')
        };
      }
    } catch (e) {
      // Command not found or failed, try next
    }
  }
  
  return {
    available: false,
    command: null,
    version: null
  };
}

/**
 * Get the Python command to use, or null if not available.
 * @returns {string|null}
 */
function getPythonCmd() {
  const result = checkPython();
  return result.command;
}

/**
 * Print a warning if Python is not available.
 * @returns {boolean} true if Python is available
 */
function warnIfNoPython() {
  const result = checkPython();
  
  if (!result.available) {
    console.warn(`
WARNING: Python 3 is not available.
Kratos memory features require Python 3 to function.

To install Python:
- Windows: https://www.python.org/downloads/
- macOS: brew install python3
- Linux: apt install python3 / yum install python3

Memory recording will be disabled until Python is available.
`);
    return false;
  }
  
  return true;
}

module.exports = {
  checkPython,
  getPythonCmd,
  warnIfNoPython
};

#!/usr/bin/env node
/**
 * Kratos Memory - Hook Installer
 * 
 * Installs Kratos hooks to ~/.claude/hooks/kratos/ and registers them
 * in ~/.claude/settings.json for global operation.
 * 
 * Usage:
 *   node install.cjs              # Install hooks
 *   node install.cjs --uninstall  # Remove hooks (keeps database)
 *   node install.cjs --status     # Check installation status
 */

const fs = require('fs');
const path = require('path');
const os = require('os');
const { checkPython, warnIfNoPython } = require('./check-python.cjs');

// Paths
const HOME = os.homedir();
const CLAUDE_DIR = path.join(HOME, '.claude');
const HOOKS_DIR = path.join(CLAUDE_DIR, 'hooks', 'kratos');
const SETTINGS_FILE = path.join(CLAUDE_DIR, 'settings.json');
const KRATOS_HOME = path.join(HOME, '.kratos');

// Source paths (relative to this script)
const SOURCE_DIR = __dirname;
const SOURCE_MEMORY_DIR = path.join(SOURCE_DIR, '..', 'memory');

// Files to copy
const HOOK_FILES = [
  'session-start.cjs',
  'session-end.cjs',
  'tool-use.cjs',
  'check-python.cjs'
];

const MEMORY_FILES = [
  'kratos_memory.py',
  'schema.sql'
];

/**
 * Ensure a directory exists
 */
function ensureDir(dirPath) {
  if (!fs.existsSync(dirPath)) {
    fs.mkdirSync(dirPath, { recursive: true });
    return true;
  }
  return false;
}

/**
 * Copy a file, preserving content
 */
function copyFile(src, dest) {
  const content = fs.readFileSync(src);
  fs.writeFileSync(dest, content);
}

/**
 * Read JSON file safely
 */
function readJson(filePath) {
  try {
    if (fs.existsSync(filePath)) {
      return JSON.parse(fs.readFileSync(filePath, 'utf-8'));
    }
  } catch (e) {
    console.error(`Warning: Could not parse ${filePath}`);
  }
  return {};
}

/**
 * Write JSON file with pretty formatting
 */
function writeJson(filePath, data) {
  fs.writeFileSync(filePath, JSON.stringify(data, null, 2));
}

/**
 * Generate hook configuration for settings.json
 */
function generateHookConfig() {
  const hookPath = HOOKS_DIR.replace(/\\/g, '/');  // Normalize for JSON
  
  return {
    "SessionStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": `node "${hookPath}/session-start.cjs"`,
            "timeout": 5000
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Task|Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": `node "${hookPath}/tool-use.cjs"`,
            "timeout": 5000
          }
        ]
      }
    ],
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": `node "${hookPath}/session-end.cjs"`,
            "timeout": 10000
          }
        ]
      }
    ]
  };
}

/**
 * Install hooks
 */
function install() {
  console.log('Kratos Memory Hook Installer');
  console.log('============================\n');
  
  // Check Python
  const pythonInfo = checkPython();
  if (pythonInfo.available) {
    console.log(`Python: ${pythonInfo.command} (${pythonInfo.version})`);
  } else {
    warnIfNoPython();
    console.log('Continuing with installation (memory features will be disabled until Python is available).\n');
  }
  
  // Create directories
  console.log('Creating directories...');
  ensureDir(CLAUDE_DIR);
  ensureDir(HOOKS_DIR);
  ensureDir(path.join(HOOKS_DIR, 'memory'));
  ensureDir(KRATOS_HOME);
  
  // Copy hook files
  console.log('Copying hook files...');
  for (const file of HOOK_FILES) {
    const src = path.join(SOURCE_DIR, file);
    const dest = path.join(HOOKS_DIR, file);
    if (fs.existsSync(src)) {
      copyFile(src, dest);
      console.log(`  - ${file}`);
    } else {
      console.log(`  - ${file} (not found, skipping)`);
    }
  }
  
  // Copy memory files
  console.log('Copying memory system files...');
  for (const file of MEMORY_FILES) {
    const src = path.join(SOURCE_MEMORY_DIR, file);
    const dest = path.join(HOOKS_DIR, 'memory', file);
    if (fs.existsSync(src)) {
      copyFile(src, dest);
      console.log(`  - memory/${file}`);
    } else {
      console.log(`  - memory/${file} (not found, skipping)`);
    }
  }
  
  // Update settings.json
  console.log('\nUpdating settings.json...');
  const settings = readJson(SETTINGS_FILE);
  
  // Merge hooks (preserve other hooks, add/update kratos hooks)
  if (!settings.hooks) {
    settings.hooks = {};
  }
  
  const kratosHooks = generateHookConfig();
  
  // For each hook type, add or replace kratos hooks
  for (const [hookType, hookConfigs] of Object.entries(kratosHooks)) {
    if (!settings.hooks[hookType]) {
      settings.hooks[hookType] = [];
    }
    
    // Remove existing kratos hooks
    settings.hooks[hookType] = settings.hooks[hookType].filter(
      h => !h.hooks?.some(hh => hh.command?.includes('kratos'))
    );
    
    // Add new kratos hooks
    settings.hooks[hookType].push(...hookConfigs);
  }
  
  writeJson(SETTINGS_FILE, settings);
  console.log('  - Updated ~/.claude/settings.json');
  
  // Summary
  console.log('\n============================');
  console.log('Installation complete!');
  console.log(`\nHooks installed to: ${HOOKS_DIR}`);
  console.log(`Memory database: ${path.join(KRATOS_HOME, 'memory.db')}`);
  console.log('\nKratos will now track your sessions automatically.');
  console.log('Use /kratos:recall to see your last session context.');
}

/**
 * Uninstall hooks (keeps database)
 */
function uninstall() {
  console.log('Kratos Memory Hook Uninstaller');
  console.log('==============================\n');
  
  // Remove hooks from settings.json
  console.log('Updating settings.json...');
  const settings = readJson(SETTINGS_FILE);
  
  if (settings.hooks) {
    for (const hookType of Object.keys(settings.hooks)) {
      settings.hooks[hookType] = settings.hooks[hookType].filter(
        h => !h.hooks?.some(hh => hh.command?.includes('kratos'))
      );
      
      // Remove empty arrays
      if (settings.hooks[hookType].length === 0) {
        delete settings.hooks[hookType];
      }
    }
    
    // Remove empty hooks object
    if (Object.keys(settings.hooks).length === 0) {
      delete settings.hooks;
    }
    
    writeJson(SETTINGS_FILE, settings);
    console.log('  - Removed kratos hooks from settings');
  }
  
  // Remove hook files
  console.log('\nRemoving hook files...');
  if (fs.existsSync(HOOKS_DIR)) {
    fs.rmSync(HOOKS_DIR, { recursive: true, force: true });
    console.log(`  - Removed ${HOOKS_DIR}`);
  } else {
    console.log('  - Hook directory not found');
  }
  
  // Note about database
  console.log('\n==============================');
  console.log('Uninstallation complete!');
  console.log(`\nNote: Memory database preserved at ${path.join(KRATOS_HOME, 'memory.db')}`);
  console.log('To delete all data, manually remove the ~/.kratos directory.');
}

/**
 * Check installation status
 */
function status() {
  console.log('Kratos Memory Installation Status');
  console.log('==================================\n');
  
  // Python
  const pythonInfo = checkPython();
  console.log(`Python: ${pythonInfo.available ? `${pythonInfo.command} (${pythonInfo.version})` : 'NOT FOUND'}`);
  
  // Hooks directory
  const hooksInstalled = fs.existsSync(HOOKS_DIR);
  console.log(`Hooks directory: ${hooksInstalled ? 'INSTALLED' : 'NOT INSTALLED'}`);
  
  if (hooksInstalled) {
    console.log(`  Location: ${HOOKS_DIR}`);
    const files = fs.readdirSync(HOOKS_DIR);
    console.log(`  Files: ${files.join(', ')}`);
  }
  
  // Settings.json
  const settings = readJson(SETTINGS_FILE);
  const hasKratosHooks = settings.hooks && 
    Object.values(settings.hooks).some(hooks => 
      hooks.some(h => h.hooks?.some(hh => hh.command?.includes('kratos')))
    );
  console.log(`Settings.json: ${hasKratosHooks ? 'CONFIGURED' : 'NOT CONFIGURED'}`);
  
  // Database
  const dbPath = path.join(KRATOS_HOME, 'memory.db');
  const dbExists = fs.existsSync(dbPath);
  console.log(`Memory database: ${dbExists ? 'EXISTS' : 'NOT INITIALIZED'}`);
  if (dbExists) {
    const stats = fs.statSync(dbPath);
    console.log(`  Size: ${(stats.size / 1024).toFixed(1)} KB`);
  }
  
  // Overall status
  console.log('\n==================================');
  if (hooksInstalled && hasKratosHooks && pythonInfo.available) {
    console.log('Status: FULLY OPERATIONAL');
  } else if (hooksInstalled && hasKratosHooks) {
    console.log('Status: INSTALLED (Python missing - memory disabled)');
  } else {
    console.log('Status: NOT INSTALLED');
    console.log('\nRun "node install.cjs" to install.');
  }
}

// Main
const args = process.argv.slice(2);

if (args.includes('--uninstall') || args.includes('-u')) {
  uninstall();
} else if (args.includes('--status') || args.includes('-s')) {
  status();
} else if (args.includes('--help') || args.includes('-h')) {
  console.log(`
Kratos Memory Hook Installer

Usage:
  node install.cjs              Install hooks globally
  node install.cjs --uninstall  Remove hooks (keeps database)
  node install.cjs --status     Check installation status
  node install.cjs --help       Show this help

The installer will:
1. Copy hook files to ~/.claude/hooks/kratos/
2. Copy memory system to ~/.claude/hooks/kratos/memory/
3. Update ~/.claude/settings.json with hook configuration

Memory data is stored in ~/.kratos/memory.db (preserved on uninstall).
`);
} else {
  install();
}

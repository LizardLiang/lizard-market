-- Kratos Memory System Schema
-- Lightweight, fast memory for tracking agent journeys
-- Storage: .claude/.kratos/memory.db

PRAGMA journal_mode = WAL;          -- Write-ahead logging for performance
PRAGMA synchronous = NORMAL;        -- Balance safety and speed
PRAGMA foreign_keys = ON;           -- Enforce relationships

-- Schema version tracking
CREATE TABLE IF NOT EXISTS schema_version (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    version INTEGER NOT NULL DEFAULT 1,
    upgraded_at TEXT DEFAULT (datetime('now'))
);

INSERT OR IGNORE INTO schema_version (id, version) VALUES (1, 1);

-- Sessions: Each time Kratos is invoked
CREATE TABLE IF NOT EXISTS sessions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT UNIQUE NOT NULL,           -- UUID for the session
    project TEXT NOT NULL,                     -- Project/repo name
    feature_name TEXT,                         -- Associated feature (if any)
    initial_request TEXT,                      -- What the user asked for
    started_at INTEGER NOT NULL,               -- Unix epoch ms
    ended_at INTEGER,                          -- Unix epoch ms (null if active)
    status TEXT DEFAULT 'active',              -- active, completed, abandoned
    summary TEXT,                              -- Session summary (filled on end)
    total_steps INTEGER DEFAULT 0,
    total_agents_spawned INTEGER DEFAULT 0
);

-- Steps: Every action taken during a session
CREATE TABLE IF NOT EXISTS steps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    step_number INTEGER NOT NULL,              -- Sequential within session
    step_type TEXT NOT NULL,                   -- agent_spawn, file_modify, decision, command, error
    timestamp INTEGER NOT NULL,                -- Unix epoch ms

    -- Agent-related fields
    agent_name TEXT,                           -- metis, athena, hephaestus, etc.
    agent_model TEXT,                          -- opus, sonnet, haiku
    pipeline_stage INTEGER,                    -- 0-8

    -- Action details
    action TEXT NOT NULL,                      -- What happened
    target TEXT,                               -- File, feature, or target of action
    result TEXT,                               -- Success/failure/outcome

    -- File changes (JSON array)
    files_created TEXT,                        -- ["path1", "path2"]
    files_modified TEXT,                       -- ["path1", "path2"]
    files_deleted TEXT,                        -- ["path1", "path2"]

    -- Context
    context TEXT,                              -- Additional context/notes

    FOREIGN KEY (session_id) REFERENCES sessions(session_id)
);

-- Features: Track feature pipeline progress
CREATE TABLE IF NOT EXISTS features (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    feature_name TEXT UNIQUE NOT NULL,
    project TEXT NOT NULL,
    created_at INTEGER NOT NULL,               -- Unix epoch ms
    updated_at INTEGER NOT NULL,               -- Unix epoch ms
    current_stage INTEGER DEFAULT 0,           -- 0-8
    status TEXT DEFAULT 'in_progress',         -- in_progress, completed, abandoned

    -- Stage completion tracking
    stage_0_completed INTEGER,                 -- Unix epoch ms
    stage_1_completed INTEGER,
    stage_2_completed INTEGER,
    stage_3_completed INTEGER,
    stage_4_completed INTEGER,
    stage_5_completed INTEGER,
    stage_6_completed INTEGER,
    stage_7_completed INTEGER,
    stage_8_completed INTEGER,

    -- Documents created
    prd_path TEXT,
    tech_spec_path TEXT,
    test_plan_path TEXT,
    implementation_notes_path TEXT,

    -- Summary
    description TEXT,
    total_sessions INTEGER DEFAULT 0,
    total_agents_used INTEGER DEFAULT 0
);

-- File changelog: Track all file modifications
CREATE TABLE IF NOT EXISTS file_changes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    step_id INTEGER,
    timestamp INTEGER NOT NULL,                -- Unix epoch ms
    file_path TEXT NOT NULL,
    change_type TEXT NOT NULL,                 -- created, modified, deleted, renamed
    old_path TEXT,                             -- For renames
    description TEXT,                          -- What changed
    lines_added INTEGER,
    lines_removed INTEGER,

    FOREIGN KEY (session_id) REFERENCES sessions(session_id),
    FOREIGN KEY (step_id) REFERENCES steps(id)
);

-- Decisions: Important choices made
CREATE TABLE IF NOT EXISTS decisions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    step_id INTEGER,
    feature_name TEXT,
    timestamp INTEGER NOT NULL,                -- Unix epoch ms
    decision_type TEXT NOT NULL,               -- architecture, implementation, trade_off, direction
    question TEXT NOT NULL,                    -- What was decided
    choice TEXT NOT NULL,                      -- What was chosen
    alternatives TEXT,                         -- JSON array of alternatives considered
    rationale TEXT,                            -- Why this choice
    impact TEXT,                               -- Expected impact

    FOREIGN KEY (session_id) REFERENCES sessions(session_id),
    FOREIGN KEY (step_id) REFERENCES steps(id)
);

-- Indexes for fast queries
CREATE INDEX IF NOT EXISTS idx_sessions_project ON sessions(project);
CREATE INDEX IF NOT EXISTS idx_sessions_feature ON sessions(feature_name);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_sessions_started ON sessions(started_at DESC);

CREATE INDEX IF NOT EXISTS idx_steps_session ON steps(session_id);
CREATE INDEX IF NOT EXISTS idx_steps_type ON steps(step_type);
CREATE INDEX IF NOT EXISTS idx_steps_agent ON steps(agent_name);
CREATE INDEX IF NOT EXISTS idx_steps_timestamp ON steps(timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_features_project ON features(project);
CREATE INDEX IF NOT EXISTS idx_features_status ON features(status);

CREATE INDEX IF NOT EXISTS idx_file_changes_session ON file_changes(session_id);
CREATE INDEX IF NOT EXISTS idx_file_changes_path ON file_changes(file_path);
CREATE INDEX IF NOT EXISTS idx_file_changes_timestamp ON file_changes(timestamp DESC);

CREATE INDEX IF NOT EXISTS idx_decisions_session ON decisions(session_id);
CREATE INDEX IF NOT EXISTS idx_decisions_feature ON decisions(feature_name);
CREATE INDEX IF NOT EXISTS idx_decisions_type ON decisions(decision_type);

-- FTS5 for full-text search on steps and decisions
CREATE VIRTUAL TABLE IF NOT EXISTS steps_fts USING fts5(
    action,
    target,
    result,
    context,
    content='steps',
    content_rowid='id'
);

CREATE VIRTUAL TABLE IF NOT EXISTS decisions_fts USING fts5(
    question,
    choice,
    rationale,
    impact,
    content='decisions',
    content_rowid='id'
);

-- Triggers to keep FTS in sync
CREATE TRIGGER IF NOT EXISTS steps_ai AFTER INSERT ON steps BEGIN
    INSERT INTO steps_fts(rowid, action, target, result, context)
    VALUES (new.id, new.action, new.target, new.result, new.context);
END;

CREATE TRIGGER IF NOT EXISTS steps_ad AFTER DELETE ON steps BEGIN
    INSERT INTO steps_fts(steps_fts, rowid, action, target, result, context)
    VALUES ('delete', old.id, old.action, old.target, old.result, old.context);
END;

CREATE TRIGGER IF NOT EXISTS steps_au AFTER UPDATE ON steps BEGIN
    INSERT INTO steps_fts(steps_fts, rowid, action, target, result, context)
    VALUES ('delete', old.id, old.action, old.target, old.result, old.context);
    INSERT INTO steps_fts(rowid, action, target, result, context)
    VALUES (new.id, new.action, new.target, new.result, new.context);
END;

CREATE TRIGGER IF NOT EXISTS decisions_ai AFTER INSERT ON decisions BEGIN
    INSERT INTO decisions_fts(rowid, question, choice, rationale, impact)
    VALUES (new.id, new.question, new.choice, new.rationale, new.impact);
END;

CREATE TRIGGER IF NOT EXISTS decisions_ad AFTER DELETE ON decisions BEGIN
    INSERT INTO decisions_fts(decisions_fts, rowid, question, choice, rationale, impact)
    VALUES ('delete', old.id, old.question, old.choice, old.rationale, old.impact);
END;

CREATE TRIGGER IF NOT EXISTS decisions_au AFTER UPDATE ON decisions BEGIN
    INSERT INTO decisions_fts(decisions_fts, rowid, question, choice, rationale, impact)
    VALUES ('delete', old.id, old.question, old.choice, old.rationale, old.impact);
    INSERT INTO decisions_fts(rowid, question, choice, rationale, impact)
    VALUES (new.id, new.question, new.choice, new.rationale, new.impact);
END;

//! Kratos Memory System - Fast SQLite interface for journey tracking
//!
//! A high-performance Rust implementation for recording and querying
//! the journey of Kratos agent orchestration sessions.

use chrono::Utc;
use clap::{Parser, Subcommand};
use rusqlite::{params, Connection, Result as SqliteResult};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use std::env;
use std::fs;
use std::path::PathBuf;
use uuid::Uuid;

/// Get current timestamp in milliseconds
fn now_ms() -> i64 {
    Utc::now().timestamp_millis()
}

/// Get database path from environment or default
fn get_db_path() -> PathBuf {
    env::var("KRATOS_MEMORY_DB")
        .map(PathBuf::from)
        .unwrap_or_else(|_| {
            env::current_dir()
                .unwrap_or_default()
                .join(".claude")
                .join(".kratos")
                .join("memory.db")
        })
}

/// Ensure database directory exists
fn ensure_db_dir(path: &PathBuf) -> std::io::Result<()> {
    if let Some(parent) = path.parent() {
        fs::create_dir_all(parent)?;
    }
    Ok(())
}

/// Get optimized database connection
fn get_connection() -> SqliteResult<Connection> {
    let path = get_db_path();
    ensure_db_dir(&path).expect("Failed to create database directory");

    let conn = Connection::open(&path)?;
    conn.execute_batch(
        "PRAGMA journal_mode = WAL;
         PRAGMA synchronous = NORMAL;
         PRAGMA foreign_keys = ON;",
    )?;
    Ok(conn)
}

/// Initialize database with schema
fn init_db() -> SqliteResult<()> {
    let path = get_db_path();
    ensure_db_dir(&path).expect("Failed to create database directory");

    let conn = Connection::open(&path)?;
    conn.execute_batch(include_str!("../schema.sql"))?;

    println!("Database initialized at: {}", path.display());
    Ok(())
}

// =============================================================================
// Data Structures
// =============================================================================

#[derive(Debug, Serialize, Deserialize)]
struct Session {
    id: i64,
    session_id: String,
    project: String,
    feature_name: Option<String>,
    initial_request: Option<String>,
    started_at: i64,
    ended_at: Option<i64>,
    status: String,
    summary: Option<String>,
    total_steps: i64,
    total_agents_spawned: i64,
}

#[derive(Debug, Serialize, Deserialize)]
struct Step {
    id: i64,
    session_id: String,
    step_number: i64,
    step_type: String,
    timestamp: i64,
    agent_name: Option<String>,
    agent_model: Option<String>,
    pipeline_stage: Option<i64>,
    action: String,
    target: Option<String>,
    result: Option<String>,
    files_created: Option<String>,
    files_modified: Option<String>,
    files_deleted: Option<String>,
    context: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
struct Feature {
    id: i64,
    feature_name: String,
    project: String,
    created_at: i64,
    updated_at: i64,
    current_stage: i64,
    status: String,
}

#[derive(Debug, Serialize, Deserialize)]
struct FileChange {
    id: i64,
    session_id: String,
    step_id: Option<i64>,
    timestamp: i64,
    file_path: String,
    change_type: String,
    description: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
struct Decision {
    id: i64,
    session_id: String,
    feature_name: Option<String>,
    timestamp: i64,
    decision_type: String,
    question: String,
    choice: String,
    rationale: Option<String>,
}

// =============================================================================
// Session Operations
// =============================================================================

fn start_session(
    project: &str,
    feature_name: Option<&str>,
    initial_request: Option<&str>,
) -> SqliteResult<String> {
    let conn = get_connection()?;
    let session_id = Uuid::new_v4().to_string();

    conn.execute(
        "INSERT INTO sessions (session_id, project, feature_name, initial_request, started_at)
         VALUES (?1, ?2, ?3, ?4, ?5)",
        params![session_id, project, feature_name, initial_request, now_ms()],
    )?;

    Ok(session_id)
}

fn end_session(session_id: &str, summary: Option<&str>, status: &str) -> SqliteResult<()> {
    let conn = get_connection()?;

    // Get step counts
    let (total_steps, agents): (i64, i64) = conn.query_row(
        "SELECT COUNT(*) as total_steps,
                COUNT(CASE WHEN step_type = 'agent_spawn' THEN 1 END) as agents
         FROM steps WHERE session_id = ?1",
        params![session_id],
        |row| Ok((row.get(0)?, row.get(1)?)),
    )?;

    conn.execute(
        "UPDATE sessions
         SET ended_at = ?1, status = ?2, summary = ?3,
             total_steps = ?4, total_agents_spawned = ?5
         WHERE session_id = ?6",
        params![now_ms(), status, summary, total_steps, agents, session_id],
    )?;

    Ok(())
}

fn get_active_session(project: Option<&str>) -> SqliteResult<Option<Value>> {
    let conn = get_connection()?;

    let query = match project {
        Some(_) => {
            "SELECT * FROM sessions WHERE status = 'active' AND project = ?1
             ORDER BY started_at DESC LIMIT 1"
        }
        None => {
            "SELECT * FROM sessions WHERE status = 'active'
             ORDER BY started_at DESC LIMIT 1"
        }
    };

    let params: Vec<&dyn rusqlite::ToSql> = match project {
        Some(p) => vec![&p as &dyn rusqlite::ToSql],
        None => vec![],
    };

    let mut stmt = conn.prepare(query)?;
    let mut rows = stmt.query(params.as_slice())?;

    if let Some(row) = rows.next()? {
        Ok(Some(json!({
            "id": row.get::<_, i64>(0)?,
            "session_id": row.get::<_, String>(1)?,
            "project": row.get::<_, String>(2)?,
            "feature_name": row.get::<_, Option<String>>(3)?,
            "started_at": row.get::<_, i64>(5)?,
            "status": row.get::<_, String>(7)?
        })))
    } else {
        Ok(None)
    }
}

// =============================================================================
// Step Operations
// =============================================================================

fn record_step(
    session_id: &str,
    step_type: &str,
    action: &str,
    agent_name: Option<&str>,
    agent_model: Option<&str>,
    pipeline_stage: Option<i64>,
    target: Option<&str>,
    result: Option<&str>,
    files_created: Option<&str>,
    files_modified: Option<&str>,
    files_deleted: Option<&str>,
    context: Option<&str>,
) -> SqliteResult<i64> {
    let conn = get_connection()?;

    // Get next step number
    let step_number: i64 = conn.query_row(
        "SELECT COALESCE(MAX(step_number), 0) + 1 FROM steps WHERE session_id = ?1",
        params![session_id],
        |row| row.get(0),
    )?;

    conn.execute(
        "INSERT INTO steps (
            session_id, step_number, step_type, timestamp,
            agent_name, agent_model, pipeline_stage,
            action, target, result,
            files_created, files_modified, files_deleted, context
         ) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14)",
        params![
            session_id,
            step_number,
            step_type,
            now_ms(),
            agent_name,
            agent_model,
            pipeline_stage,
            action,
            target,
            result,
            files_created,
            files_modified,
            files_deleted,
            context
        ],
    )?;

    Ok(conn.last_insert_rowid())
}

// =============================================================================
// Feature Operations
// =============================================================================

fn create_or_update_feature(
    feature_name: &str,
    project: &str,
    current_stage: Option<i64>,
    status: Option<&str>,
    description: Option<&str>,
) -> SqliteResult<i64> {
    let conn = get_connection()?;
    let now = now_ms();

    // Check if feature exists
    let existing: Option<i64> = conn
        .query_row(
            "SELECT id FROM features WHERE feature_name = ?1",
            params![feature_name],
            |row| row.get(0),
        )
        .ok();

    if let Some(id) = existing {
        // Update existing
        let mut updates = vec!["updated_at = ?1".to_string()];
        let mut param_idx = 2;

        if current_stage.is_some() {
            updates.push(format!("current_stage = ?{}", param_idx));
            param_idx += 1;
        }
        if status.is_some() {
            updates.push(format!("status = ?{}", param_idx));
            param_idx += 1;
        }
        if description.is_some() {
            updates.push(format!("description = ?{}", param_idx));
        }

        let sql = format!(
            "UPDATE features SET {} WHERE feature_name = ?{}",
            updates.join(", "),
            param_idx
        );

        // Build params dynamically
        let mut params_vec: Vec<Box<dyn rusqlite::ToSql>> = vec![Box::new(now)];
        if let Some(s) = current_stage {
            params_vec.push(Box::new(s));
        }
        if let Some(s) = status {
            params_vec.push(Box::new(s.to_string()));
        }
        if let Some(d) = description {
            params_vec.push(Box::new(d.to_string()));
        }
        params_vec.push(Box::new(feature_name.to_string()));

        let params_refs: Vec<&dyn rusqlite::ToSql> =
            params_vec.iter().map(|b| b.as_ref()).collect();
        conn.execute(&sql, params_refs.as_slice())?;

        Ok(id)
    } else {
        // Insert new
        conn.execute(
            "INSERT INTO features (feature_name, project, created_at, updated_at,
                                   current_stage, status, description)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)",
            params![
                feature_name,
                project,
                now,
                now,
                current_stage.unwrap_or(0),
                status.unwrap_or("in_progress"),
                description
            ],
        )?;

        Ok(conn.last_insert_rowid())
    }
}

fn mark_stage_complete(feature_name: &str, stage: i64) -> SqliteResult<()> {
    let conn = get_connection()?;
    let now = now_ms();

    let sql = format!(
        "UPDATE features SET stage_{}_completed = ?1, current_stage = ?2, updated_at = ?3
         WHERE feature_name = ?4",
        stage
    );

    conn.execute(&sql, params![now, stage, now, feature_name])?;
    Ok(())
}

// =============================================================================
// File Change Operations
// =============================================================================

fn record_file_change(
    session_id: &str,
    file_path: &str,
    change_type: &str,
    step_id: Option<i64>,
    description: Option<&str>,
) -> SqliteResult<i64> {
    let conn = get_connection()?;

    conn.execute(
        "INSERT INTO file_changes (session_id, step_id, timestamp, file_path, change_type, description)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6)",
        params![session_id, step_id, now_ms(), file_path, change_type, description],
    )?;

    Ok(conn.last_insert_rowid())
}

// =============================================================================
// Decision Operations
// =============================================================================

fn record_decision(
    session_id: &str,
    question: &str,
    choice: &str,
    decision_type: &str,
    feature_name: Option<&str>,
    rationale: Option<&str>,
) -> SqliteResult<i64> {
    let conn = get_connection()?;

    conn.execute(
        "INSERT INTO decisions (session_id, feature_name, timestamp, decision_type, question, choice, rationale)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7)",
        params![session_id, feature_name, now_ms(), decision_type, question, choice, rationale],
    )?;

    Ok(conn.last_insert_rowid())
}

// =============================================================================
// Query Operations
// =============================================================================

fn get_journey_summary(project: Option<&str>, days: i64) -> SqliteResult<Value> {
    let conn = get_connection()?;
    let cutoff = now_ms() - (days * 24 * 60 * 60 * 1000);

    let project_filter = if project.is_some() {
        "AND project = ?2"
    } else {
        ""
    };

    // Session stats
    let session_query = format!(
        "SELECT COUNT(*) as total,
                COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed,
                COALESCE(SUM(total_steps), 0) as steps,
                COALESCE(SUM(total_agents_spawned), 0) as agents
         FROM sessions WHERE started_at > ?1 {}",
        project_filter
    );

    let session_stats: (i64, i64, i64, i64) = if let Some(p) = project {
        conn.query_row(&session_query, params![cutoff, p], |row| {
            Ok((row.get(0)?, row.get(1)?, row.get(2)?, row.get(3)?))
        })?
    } else {
        conn.query_row(&session_query, params![cutoff], |row| {
            Ok((row.get(0)?, row.get(1)?, row.get(2)?, row.get(3)?))
        })?
    };

    // Feature stats
    let feature_query = format!(
        "SELECT COUNT(*) as total,
                COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed
         FROM features WHERE created_at > ?1 {}",
        project_filter
    );

    let feature_stats: (i64, i64) = if let Some(p) = project {
        conn.query_row(&feature_query, params![cutoff, p], |row| {
            Ok((row.get(0)?, row.get(1)?))
        })?
    } else {
        conn.query_row(&feature_query, params![cutoff], |row| {
            Ok((row.get(0)?, row.get(1)?))
        })?
    };

    Ok(json!({
        "period_days": days,
        "project": project.unwrap_or("all"),
        "sessions": {
            "total": session_stats.0,
            "completed": session_stats.1,
            "total_steps": session_stats.2,
            "total_agents": session_stats.3
        },
        "features": {
            "total": feature_stats.0,
            "completed": feature_stats.1
        }
    }))
}

// =============================================================================
// CLI
// =============================================================================

#[derive(Parser)]
#[command(name = "kratos-memory")]
#[command(about = "Fast SQLite memory system for Kratos agent orchestrator")]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Initialize the database
    Init,

    /// Session management
    Session {
        #[command(subcommand)]
        action: SessionAction,
    },

    /// Record a step
    Step {
        session_id: String,
        step_type: String,
        action: String,
        #[arg(long)]
        agent: Option<String>,
        #[arg(long)]
        model: Option<String>,
        #[arg(long)]
        stage: Option<i64>,
        #[arg(long)]
        target: Option<String>,
        #[arg(long)]
        result: Option<String>,
    },

    /// Feature management
    Feature {
        #[command(subcommand)]
        action: FeatureAction,
    },

    /// Record a file change
    FileChange {
        session_id: String,
        path: String,
        change_type: String,
        #[arg(long)]
        description: Option<String>,
    },

    /// Record a decision
    Decision {
        session_id: String,
        question: String,
        choice: String,
        #[arg(long, default_value = "implementation")]
        decision_type: String,
        #[arg(long)]
        feature: Option<String>,
        #[arg(long)]
        rationale: Option<String>,
    },

    /// Get journey summary
    Summary {
        #[arg(long)]
        project: Option<String>,
        #[arg(long, default_value = "30")]
        days: i64,
    },
}

#[derive(Subcommand)]
enum SessionAction {
    /// Start a new session
    Start {
        project: String,
        #[arg(long)]
        feature: Option<String>,
        #[arg(long)]
        request: Option<String>,
    },
    /// End a session
    End {
        session_id: String,
        #[arg(long)]
        summary: Option<String>,
        #[arg(long, default_value = "completed")]
        status: String,
    },
    /// Get active session
    Active {
        #[arg(long)]
        project: Option<String>,
    },
}

#[derive(Subcommand)]
enum FeatureAction {
    /// Create or update a feature
    Create {
        name: String,
        project: String,
        #[arg(long)]
        stage: Option<i64>,
        #[arg(long)]
        status: Option<String>,
    },
    /// Mark a stage complete
    Stage { name: String, stage: i64 },
}

fn main() {
    let cli = Cli::parse();

    let result = match cli.command {
        Commands::Init => {
            init_db().map(|_| json!({"status": "initialized"}))
        }

        Commands::Session { action } => match action {
            SessionAction::Start {
                project,
                feature,
                request,
            } => {
                start_session(&project, feature.as_deref(), request.as_deref())
                    .map(|id| json!({"session_id": id}))
            }
            SessionAction::End {
                session_id,
                summary,
                status,
            } => {
                end_session(&session_id, summary.as_deref(), &status)
                    .map(|_| json!({"status": "ended", "session_id": session_id}))
            }
            SessionAction::Active { project } => {
                get_active_session(project.as_deref()).map(|s| s.unwrap_or(json!(null)))
            }
        },

        Commands::Step {
            session_id,
            step_type,
            action,
            agent,
            model,
            stage,
            target,
            result,
        } => record_step(
            &session_id,
            &step_type,
            &action,
            agent.as_deref(),
            model.as_deref(),
            stage,
            target.as_deref(),
            result.as_deref(),
            None,
            None,
            None,
            None,
        )
        .map(|id| json!({"step_id": id})),

        Commands::Feature { action } => match action {
            FeatureAction::Create {
                name,
                project,
                stage,
                status,
            } => create_or_update_feature(&name, &project, stage, status.as_deref(), None)
                .map(|id| json!({"feature_id": id})),
            FeatureAction::Stage { name, stage } => {
                mark_stage_complete(&name, stage).map(|_| json!({"status": "stage_marked"}))
            }
        },

        Commands::FileChange {
            session_id,
            path,
            change_type,
            description,
        } => record_file_change(&session_id, &path, &change_type, None, description.as_deref())
            .map(|id| json!({"change_id": id})),

        Commands::Decision {
            session_id,
            question,
            choice,
            decision_type,
            feature,
            rationale,
        } => record_decision(
            &session_id,
            &question,
            &choice,
            &decision_type,
            feature.as_deref(),
            rationale.as_deref(),
        )
        .map(|id| json!({"decision_id": id})),

        Commands::Summary { project, days } => get_journey_summary(project.as_deref(), days),
    };

    match result {
        Ok(value) => println!("{}", serde_json::to_string_pretty(&value).unwrap()),
        Err(e) => {
            eprintln!("{}", json!({"error": e.to_string()}));
            std::process::exit(1);
        }
    }
}

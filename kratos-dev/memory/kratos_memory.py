#!/usr/bin/env python3
"""
Kratos Memory System - Fast SQLite interface for journey tracking.

Usage:
    python kratos_memory.py init                              # Initialize database
    python kratos_memory.py session start <project> [feature] # Start session
    python kratos_memory.py session end <session_id> [summary] # End session
    python kratos_memory.py step <session_id> <type> <action> [options] # Record step
    python kratos_memory.py query <type> [filters]            # Query memory
    python kratos_memory.py feature <action> [args]           # Feature operations
    python kratos_memory.py file-change <session_id> <path> <type> [desc] # Record file change
    python kratos_memory.py decision <session_id> <question> <choice> [opts] # Record decision
    python kratos_memory.py summary [project] [days]          # Get journey summary
    python kratos_memory.py last-session [project] [--global] [--format=json|text] # Get last session info
"""

import sqlite3
import json
import os
import sys
import uuid
import time
from pathlib import Path
from typing import Optional, List, Dict, Any
from dataclasses import dataclass, asdict
from datetime import datetime, timedelta


# Default database path - GLOBAL location like Claude-Mem
def get_default_db_path() -> Path:
    """Get default global database path."""
    home = Path.home()
    return home / ".kratos" / "memory.db"


DEFAULT_DB_PATH = get_default_db_path()
SCHEMA_PATH = Path(__file__).parent / "schema.sql"


def get_db_path() -> Path:
    """Get database path, allowing override via environment variable."""
    return Path(os.environ.get("KRATOS_MEMORY_DB", DEFAULT_DB_PATH))


def ensure_db_dir(db_path: Path) -> None:
    """Ensure database directory exists."""
    db_path.parent.mkdir(parents=True, exist_ok=True)


def get_connection(db_path: Optional[Path] = None) -> sqlite3.Connection:
    """Get database connection with optimal settings."""
    path = db_path or get_db_path()
    ensure_db_dir(path)

    conn = sqlite3.connect(str(path), timeout=10.0)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA journal_mode = WAL")
    conn.execute("PRAGMA synchronous = NORMAL")
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


def init_db(db_path: Optional[Path] = None) -> bool:
    """Initialize database with schema."""
    path = db_path or get_db_path()
    ensure_db_dir(path)

    try:
        with open(SCHEMA_PATH, "r") as f:
            schema = f.read()

        conn = get_connection(path)
        conn.executescript(schema)
        conn.commit()
        conn.close()

        print(f"Database initialized at: {path}")
        return True
    except Exception as e:
        print(f"Error initializing database: {e}", file=sys.stderr)
        return False


def now_ms() -> int:
    """Get current time in milliseconds."""
    return int(time.time() * 1000)


# =============================================================================
# Session Management
# =============================================================================


def start_session(
    project: str,
    feature_name: Optional[str] = None,
    initial_request: Optional[str] = None,
) -> str:
    """Start a new session and return session_id."""
    session_id = str(uuid.uuid4())

    conn = get_connection()
    conn.execute(
        """
        INSERT INTO sessions (session_id, project, feature_name, initial_request, started_at)
        VALUES (?, ?, ?, ?, ?)
    """,
        (session_id, project, feature_name, initial_request, now_ms()),
    )
    conn.commit()
    conn.close()

    return session_id


def end_session(
    session_id: str, summary: Optional[str] = None, status: str = "completed"
) -> bool:
    """End a session and record summary."""
    conn = get_connection()

    # Get step counts
    cursor = conn.execute(
        """
        SELECT COUNT(*) as total_steps,
               COUNT(CASE WHEN step_type = 'agent_spawn' THEN 1 END) as agents
        FROM steps WHERE session_id = ?
    """,
        (session_id,),
    )
    row = cursor.fetchone()

    conn.execute(
        """
        UPDATE sessions
        SET ended_at = ?, status = ?, summary = ?,
            total_steps = ?, total_agents_spawned = ?
        WHERE session_id = ?
    """,
        (now_ms(), status, summary, row["total_steps"], row["agents"], session_id),
    )
    conn.commit()
    conn.close()

    return True


def get_active_session(project: Optional[str] = None) -> Optional[Dict[str, Any]]:
    """Get the most recent active session."""
    conn = get_connection()

    query = "SELECT * FROM sessions WHERE status = 'active'"
    params = []

    if project:
        query += " AND project = ?"
        params.append(project)

    query += " ORDER BY started_at DESC LIMIT 1"

    cursor = conn.execute(query, params)
    row = cursor.fetchone()
    conn.close()

    return dict(row) if row else None


# =============================================================================
# Step Recording
# =============================================================================


def record_step(
    session_id: str,
    step_type: str,
    action: str,
    agent_name: Optional[str] = None,
    agent_model: Optional[str] = None,
    pipeline_stage: Optional[int] = None,
    target: Optional[str] = None,
    result: Optional[str] = None,
    files_created: Optional[List[str]] = None,
    files_modified: Optional[List[str]] = None,
    files_deleted: Optional[List[str]] = None,
    context: Optional[str] = None,
) -> int:
    """Record a step in the journey."""
    conn = get_connection()

    # Get next step number
    cursor = conn.execute(
        "SELECT COALESCE(MAX(step_number), 0) + 1 FROM steps WHERE session_id = ?",
        (session_id,),
    )
    step_number = cursor.fetchone()[0]

    cursor = conn.execute(
        """
        INSERT INTO steps (
            session_id, step_number, step_type, timestamp,
            agent_name, agent_model, pipeline_stage,
            action, target, result,
            files_created, files_modified, files_deleted, context
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    """,
        (
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
            json.dumps(files_created) if files_created else None,
            json.dumps(files_modified) if files_modified else None,
            json.dumps(files_deleted) if files_deleted else None,
            context,
        ),
    )

    step_id = cursor.lastrowid
    conn.commit()
    conn.close()

    return step_id


# =============================================================================
# Feature Tracking
# =============================================================================


def create_or_update_feature(
    feature_name: str,
    project: str,
    current_stage: Optional[int] = None,
    status: Optional[str] = None,
    description: Optional[str] = None,
) -> int:
    """Create or update a feature."""
    conn = get_connection()
    now = now_ms()

    # Check if feature exists
    cursor = conn.execute(
        "SELECT id FROM features WHERE feature_name = ?", (feature_name,)
    )
    row = cursor.fetchone()

    if row:
        # Update existing
        updates = ["updated_at = ?"]
        params = [now]

        if current_stage is not None:
            updates.append("current_stage = ?")
            params.append(current_stage)
            updates.append(f"stage_{current_stage}_completed = ?")
            params.append(now)

        if status:
            updates.append("status = ?")
            params.append(status)

        if description:
            updates.append("description = ?")
            params.append(description)

        params.append(feature_name)
        conn.execute(
            f"""
            UPDATE features SET {", ".join(updates)}
            WHERE feature_name = ?
        """,
            params,
        )
        feature_id = row["id"]
    else:
        # Insert new
        cursor = conn.execute(
            """
            INSERT INTO features (feature_name, project, created_at, updated_at,
                                  current_stage, status, description)
            VALUES (?, ?, ?, ?, ?, ?, ?)
        """,
            (
                feature_name,
                project,
                now,
                now,
                current_stage or 0,
                status or "in_progress",
                description,
            ),
        )
        feature_id = cursor.lastrowid

    conn.commit()
    conn.close()

    return feature_id


def get_feature(feature_name: str) -> Optional[Dict[str, Any]]:
    """Get feature details."""
    conn = get_connection()
    cursor = conn.execute(
        "SELECT * FROM features WHERE feature_name = ?", (feature_name,)
    )
    row = cursor.fetchone()
    conn.close()

    return dict(row) if row else None


def mark_stage_complete(feature_name: str, stage: int) -> bool:
    """Mark a pipeline stage as complete."""
    conn = get_connection()
    conn.execute(
        f"""
        UPDATE features
        SET stage_{stage}_completed = ?, current_stage = ?, updated_at = ?
        WHERE feature_name = ?
    """,
        (now_ms(), stage, now_ms(), feature_name),
    )
    conn.commit()
    conn.close()
    return True


# =============================================================================
# File Change Tracking
# =============================================================================


def record_file_change(
    session_id: str,
    file_path: str,
    change_type: str,
    step_id: Optional[int] = None,
    old_path: Optional[str] = None,
    description: Optional[str] = None,
    lines_added: Optional[int] = None,
    lines_removed: Optional[int] = None,
) -> int:
    """Record a file change."""
    conn = get_connection()
    cursor = conn.execute(
        """
        INSERT INTO file_changes (
            session_id, step_id, timestamp, file_path, change_type,
            old_path, description, lines_added, lines_removed
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
    """,
        (
            session_id,
            step_id,
            now_ms(),
            file_path,
            change_type,
            old_path,
            description,
            lines_added,
            lines_removed,
        ),
    )

    change_id = cursor.lastrowid
    conn.commit()
    conn.close()

    return change_id


# =============================================================================
# Decision Recording
# =============================================================================


def record_decision(
    session_id: str,
    question: str,
    choice: str,
    decision_type: str = "implementation",
    step_id: Optional[int] = None,
    feature_name: Optional[str] = None,
    alternatives: Optional[List[str]] = None,
    rationale: Optional[str] = None,
    impact: Optional[str] = None,
) -> int:
    """Record a decision."""
    conn = get_connection()
    cursor = conn.execute(
        """
        INSERT INTO decisions (
            session_id, step_id, feature_name, timestamp,
            decision_type, question, choice, alternatives, rationale, impact
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    """,
        (
            session_id,
            step_id,
            feature_name,
            now_ms(),
            decision_type,
            question,
            choice,
            json.dumps(alternatives) if alternatives else None,
            rationale,
            impact,
        ),
    )

    decision_id = cursor.lastrowid
    conn.commit()
    conn.close()

    return decision_id


# =============================================================================
# Query Functions
# =============================================================================


def get_recent_sessions(
    project: Optional[str] = None, days: int = 7, limit: int = 20
) -> List[Dict[str, Any]]:
    """Get recent sessions."""
    conn = get_connection()

    cutoff = now_ms() - (days * 24 * 60 * 60 * 1000)

    query = "SELECT * FROM sessions WHERE started_at > ?"
    params = [cutoff]

    if project:
        query += " AND project = ?"
        params.append(project)

    query += " ORDER BY started_at DESC LIMIT ?"
    params.append(limit)

    cursor = conn.execute(query, params)
    rows = cursor.fetchall()
    conn.close()

    return [dict(row) for row in rows]


def get_session_steps(session_id: str) -> List[Dict[str, Any]]:
    """Get all steps for a session."""
    conn = get_connection()
    cursor = conn.execute(
        """
        SELECT * FROM steps WHERE session_id = ?
        ORDER BY step_number ASC
    """,
        (session_id,),
    )
    rows = cursor.fetchall()
    conn.close()

    return [dict(row) for row in rows]


def get_recent_file_changes(days: int = 7, limit: int = 50) -> List[Dict[str, Any]]:
    """Get recent file changes."""
    conn = get_connection()
    cutoff = now_ms() - (days * 24 * 60 * 60 * 1000)

    cursor = conn.execute(
        """
        SELECT * FROM file_changes WHERE timestamp > ?
        ORDER BY timestamp DESC LIMIT ?
    """,
        (cutoff, limit),
    )
    rows = cursor.fetchall()
    conn.close()

    return [dict(row) for row in rows]


def get_feature_decisions(feature_name: str) -> List[Dict[str, Any]]:
    """Get all decisions for a feature."""
    conn = get_connection()
    cursor = conn.execute(
        """
        SELECT * FROM decisions WHERE feature_name = ?
        ORDER BY timestamp ASC
    """,
        (feature_name,),
    )
    rows = cursor.fetchall()
    conn.close()

    return [dict(row) for row in rows]


def search_steps(query: str, limit: int = 20) -> List[Dict[str, Any]]:
    """Full-text search on steps."""
    conn = get_connection()
    cursor = conn.execute(
        """
        SELECT s.* FROM steps s
        WHERE s.id IN (
            SELECT rowid FROM steps_fts WHERE steps_fts MATCH ?
        )
        ORDER BY s.timestamp DESC LIMIT ?
    """,
        (query, limit),
    )
    rows = cursor.fetchall()
    conn.close()

    return [dict(row) for row in rows]


def search_decisions(query: str, limit: int = 20) -> List[Dict[str, Any]]:
    """Full-text search on decisions."""
    conn = get_connection()
    cursor = conn.execute(
        """
        SELECT d.* FROM decisions d
        WHERE d.id IN (
            SELECT rowid FROM decisions_fts WHERE decisions_fts MATCH ?
        )
        ORDER BY d.timestamp DESC LIMIT ?
    """,
        (query, limit),
    )
    rows = cursor.fetchall()
    conn.close()

    return [dict(row) for row in rows]


# =============================================================================
# Last Session Query (for recall feature)
# =============================================================================

# Stage name mapping for display
STAGE_NAMES = {
    0: "Research",
    1: "PRD Creation",
    2: "PRD Review",
    3: "Tech Spec",
    4: "PM Spec Review",
    5: "SA Spec Review",
    6: "Test Plan",
    7: "Implementation",
    8: "Code Review",
}

STAGE_AGENTS = {
    0: "Metis",
    1: "Athena",
    2: "Athena",
    3: "Hephaestus",
    4: "Athena",
    5: "Apollo",
    6: "Artemis",
    7: "Ares",
    8: "Hermes",
}


def get_last_session_info(
    project: Optional[str] = None, global_mode: bool = False
) -> Optional[Dict[str, Any]]:
    """
    Get comprehensive information about the last session.

    Returns structured data suitable for recall display.
    """
    conn = get_connection()

    # Build query based on mode
    if global_mode:
        # Get recent sessions across all projects
        cursor = conn.execute("""
            SELECT * FROM sessions
            ORDER BY started_at DESC
            LIMIT 10
        """)
        sessions = [dict(row) for row in cursor.fetchall()]

        if not sessions:
            conn.close()
            return None

        # For global mode, return list of sessions
        result = {"mode": "global", "sessions": []}

        for sess in sessions:
            feature_info = None
            if sess.get("feature_name"):
                cursor = conn.execute(
                    "SELECT * FROM features WHERE feature_name = ?",
                    (sess["feature_name"],),
                )
                feat_row = cursor.fetchone()
                if feat_row:
                    feature_info = dict(feat_row)

            result["sessions"].append(
                {
                    "session_id": sess["session_id"],
                    "project": sess["project"],
                    "feature_name": sess.get("feature_name"),
                    "status": sess["status"],
                    "started_at": sess["started_at"],
                    "ended_at": sess.get("ended_at"),
                    "current_stage": feature_info["current_stage"]
                    if feature_info
                    else None,
                    "feature_status": feature_info["status"] if feature_info else None,
                }
            )

        conn.close()
        return result

    # Project-specific mode
    query = "SELECT * FROM sessions"
    params = []

    if project:
        query += " WHERE project = ?"
        params.append(project)

    query += " ORDER BY started_at DESC LIMIT 1"

    cursor = conn.execute(query, params)
    session = cursor.fetchone()

    if not session:
        conn.close()
        return None

    session = dict(session)

    # Get feature info if exists
    feature_info = None
    if session.get("feature_name"):
        cursor = conn.execute(
            "SELECT * FROM features WHERE feature_name = ?", (session["feature_name"],)
        )
        feat_row = cursor.fetchone()
        if feat_row:
            feature_info = dict(feat_row)

    # Get last 5 steps
    cursor = conn.execute(
        """
        SELECT * FROM steps
        WHERE session_id = ?
        ORDER BY step_number DESC
        LIMIT 5
    """,
        (session["session_id"],),
    )
    recent_steps = [dict(row) for row in cursor.fetchall()]
    recent_steps.reverse()  # Oldest first

    # Get last decision
    cursor = conn.execute(
        """
        SELECT * FROM decisions
        WHERE session_id = ?
        ORDER BY timestamp DESC
        LIMIT 1
    """,
        (session["session_id"],),
    )
    last_decision_row = cursor.fetchone()
    last_decision = dict(last_decision_row) if last_decision_row else None

    conn.close()

    # Build result
    current_stage = feature_info["current_stage"] if feature_info else 0
    next_stage = min(current_stage + 1, 8) if current_stage < 8 else None

    # Format last actions
    last_actions = []
    for step in recent_steps:
        action_str = step.get("action", "")
        agent = step.get("agent_name", "")
        if agent:
            action_str = f"{agent.capitalize()}: {action_str}"
        last_actions.append(action_str)

    # Build recommendation
    recommendation = None
    if (
        feature_info
        and feature_info["status"] == "in_progress"
        and next_stage is not None
    ):
        next_agent = STAGE_AGENTS.get(next_stage, "Unknown")
        next_stage_name = STAGE_NAMES.get(next_stage, "Unknown")
        recommendation = (
            f"Continue with Stage {next_stage} ({next_agent} - {next_stage_name})?"
        )
    elif feature_info and feature_info["status"] == "completed":
        recommendation = "Feature completed! Start a new one with /kratos:start"

    return {
        "mode": "project",
        "session_id": session["session_id"],
        "project": session["project"],
        "feature_name": session.get("feature_name"),
        "current_stage": current_stage,
        "stage_name": STAGE_NAMES.get(current_stage, "Unknown"),
        "next_stage": next_stage,
        "next_stage_name": STAGE_NAMES.get(next_stage) if next_stage else None,
        "next_agent": STAGE_AGENTS.get(next_stage) if next_stage else None,
        "started_at": session["started_at"],
        "ended_at": session.get("ended_at"),
        "status": session["status"],
        "feature_status": feature_info["status"] if feature_info else None,
        "last_actions": last_actions,
        "last_decision": last_decision,
        "recommendation": recommendation,
        "total_steps": session.get("total_steps", 0),
        "total_agents": session.get("total_agents_spawned", 0),
    }


def format_last_session_text(info: Dict[str, Any]) -> str:
    """Format last session info as human-readable text."""
    if not info:
        return "No previous sessions found."

    if info.get("mode") == "global":
        lines = [
            "KRATOS RECALL (Global)",
            "=" * 50,
            "",
            "Recent sessions across all projects:",
            "",
        ]

        for i, sess in enumerate(info["sessions"], 1):
            stage_str = (
                f"Stage {sess['current_stage']}/8"
                if sess.get("current_stage") is not None
                else ""
            )
            status_str = sess.get("feature_status", sess["status"])
            time_ago = format_time_ago(sess["started_at"])

            feature = sess.get("feature_name") or "(no feature)"
            lines.append(
                f"{i}. {sess['project']}/{feature} - {stage_str} {status_str} - {time_ago}"
            )

        lines.append("")
        lines.append("Use /kratos:recall in the project directory for details.")

        return "\n".join(lines)

    # Project-specific format
    time_ago = format_time_ago(info["started_at"])

    lines = [
        "KRATOS RECALL",
        "=" * 50,
        "",
        f"Feature: {info.get('feature_name') or '(none)'}",
        f"Stage: {info['current_stage']}/8 ({info['stage_name']})",
        f"Status: {info.get('feature_status', info['status'])}",
        f"Last active: {time_ago}",
        "",
    ]

    if info.get("last_actions"):
        lines.append("Last actions:")
        for action in info["last_actions"][-3:]:  # Show last 3
            lines.append(f"  - {action}")
        lines.append("")

    if info.get("last_decision"):
        dec = info["last_decision"]
        lines.append(f"Last decision: {dec.get('question', 'N/A')}")
        lines.append(f"  Choice: {dec.get('choice', 'N/A')}")
        lines.append("")

    # Pipeline visualization
    stages = []
    for i in range(1, 9):
        if i < info["current_stage"]:
            stages.append(f"[{i}]OK")
        elif i == info["current_stage"]:
            stages.append(f"[{i}]>>")
        else:
            stages.append(f"[{i}]..")
    lines.append("Pipeline: " + " -> ".join(stages))
    lines.append("")

    if info.get("recommendation"):
        lines.append(f"Recommendation: {info['recommendation']}")

    return "\n".join(lines)


def format_time_ago(timestamp_ms: int) -> str:
    """Format timestamp as 'X ago' string."""
    if not timestamp_ms:
        return "unknown"

    diff_ms = now_ms() - timestamp_ms
    diff_sec = diff_ms / 1000
    diff_min = diff_sec / 60
    diff_hour = diff_min / 60
    diff_day = diff_hour / 24

    if diff_min < 1:
        return "just now"
    elif diff_min < 60:
        return f"{int(diff_min)} minutes ago"
    elif diff_hour < 24:
        return f"{int(diff_hour)} hours ago"
    elif diff_day < 7:
        return f"{int(diff_day)} days ago"
    else:
        return f"{int(diff_day // 7)} weeks ago"


# =============================================================================
# Summary Generation
# =============================================================================


def get_journey_summary(
    project: Optional[str] = None, days: int = 30
) -> Dict[str, Any]:
    """Generate a journey summary."""
    conn = get_connection()
    cutoff = now_ms() - (days * 24 * 60 * 60 * 1000)

    project_filter = "AND project = ?" if project else ""
    params_base = [cutoff]
    if project:
        params_base.append(project)

    # Session stats
    cursor = conn.execute(
        f"""
        SELECT
            COUNT(*) as total_sessions,
            COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_sessions,
            SUM(total_steps) as total_steps,
            SUM(total_agents_spawned) as total_agents
        FROM sessions
        WHERE started_at > ? {project_filter}
    """,
        params_base,
    )
    session_stats = dict(cursor.fetchone())

    # Feature stats
    cursor = conn.execute(
        f"""
        SELECT
            COUNT(*) as total_features,
            COUNT(CASE WHEN status = 'completed' THEN 1 END) as completed_features,
            AVG(current_stage) as avg_stage
        FROM features
        WHERE created_at > ? {project_filter}
    """,
        params_base,
    )
    feature_stats = dict(cursor.fetchone())

    # Agent usage
    cursor = conn.execute(
        f"""
        SELECT agent_name, COUNT(*) as count
        FROM steps s
        JOIN sessions sess ON s.session_id = sess.session_id
        WHERE s.step_type = 'agent_spawn'
        AND s.timestamp > ? {project_filter}
        GROUP BY agent_name
        ORDER BY count DESC
    """,
        params_base,
    )
    agent_usage = {row["agent_name"]: row["count"] for row in cursor.fetchall()}

    # File change summary
    cursor = conn.execute(
        f"""
        SELECT change_type, COUNT(*) as count
        FROM file_changes fc
        JOIN sessions sess ON fc.session_id = sess.session_id
        WHERE fc.timestamp > ? {project_filter}
        GROUP BY change_type
    """,
        params_base,
    )
    file_changes = {row["change_type"]: row["count"] for row in cursor.fetchall()}

    # Recent decisions
    cursor = conn.execute(
        f"""
        SELECT d.question, d.choice, d.decision_type
        FROM decisions d
        JOIN sessions sess ON d.session_id = sess.session_id
        WHERE d.timestamp > ? {project_filter}
        ORDER BY d.timestamp DESC LIMIT 10
    """,
        params_base,
    )
    recent_decisions = [dict(row) for row in cursor.fetchall()]

    conn.close()

    return {
        "period_days": days,
        "project": project or "all",
        "sessions": session_stats,
        "features": feature_stats,
        "agent_usage": agent_usage,
        "file_changes": file_changes,
        "recent_decisions": recent_decisions,
    }


# =============================================================================
# CLI Interface
# =============================================================================


def main():
    """CLI entry point."""
    if len(sys.argv) < 2:
        print(__doc__)
        sys.exit(1)

    command = sys.argv[1]

    try:
        if command == "init":
            init_db()

        elif command == "session":
            if len(sys.argv) < 3:
                print("Usage: session <start|end> [args]")
                sys.exit(1)

            action = sys.argv[2]

            if action == "start":
                if len(sys.argv) < 4:
                    print("Usage: session start <project> [feature] [request]")
                    sys.exit(1)
                project = sys.argv[3]
                feature = sys.argv[4] if len(sys.argv) > 4 else None
                request = sys.argv[5] if len(sys.argv) > 5 else None
                session_id = start_session(project, feature, request)
                print(json.dumps({"session_id": session_id}))

            elif action == "end":
                if len(sys.argv) < 4:
                    print("Usage: session end <session_id> [summary] [status]")
                    sys.exit(1)
                session_id = sys.argv[3]
                summary = sys.argv[4] if len(sys.argv) > 4 else None
                status = sys.argv[5] if len(sys.argv) > 5 else "completed"
                end_session(session_id, summary, status)
                print(json.dumps({"status": "ended", "session_id": session_id}))

            elif action == "active":
                project = sys.argv[3] if len(sys.argv) > 3 else None
                session = get_active_session(project)
                print(json.dumps(session))

        elif command == "step":
            if len(sys.argv) < 5:
                print(
                    "Usage: step <session_id> <type> <action> [--agent=X] [--model=X] ..."
                )
                sys.exit(1)

            session_id = sys.argv[2]
            step_type = sys.argv[3]
            action = sys.argv[4]

            # Parse optional arguments
            kwargs = {}
            for arg in sys.argv[5:]:
                if arg.startswith("--"):
                    key, value = arg[2:].split("=", 1)
                    key = key.replace("-", "_")
                    # Map short names to function parameter names
                    key_map = {
                        "agent": "agent_name",
                        "model": "agent_model",
                        "stage": "pipeline_stage",
                    }
                    key = key_map.get(key, key)
                    if key in ["pipeline_stage", "lines_added", "lines_removed"]:
                        value = int(value)
                    elif key in ["files_created", "files_modified", "files_deleted"]:
                        value = json.loads(value)
                    kwargs[key] = value

            step_id = record_step(session_id, step_type, action, **kwargs)
            print(json.dumps({"step_id": step_id}))

        elif command == "feature":
            if len(sys.argv) < 3:
                print("Usage: feature <create|update|get|stage> [args]")
                sys.exit(1)

            action = sys.argv[2]

            if action in ["create", "update"]:
                if len(sys.argv) < 5:
                    print(
                        "Usage: feature create <name> <project> [--stage=X] [--status=X]"
                    )
                    sys.exit(1)
                name = sys.argv[3]
                project = sys.argv[4]
                kwargs = {}
                for arg in sys.argv[5:]:
                    if arg.startswith("--"):
                        key, value = arg[2:].split("=", 1)
                        if key == "stage":
                            kwargs["current_stage"] = int(value)
                        else:
                            kwargs[key.replace("-", "_")] = value
                feature_id = create_or_update_feature(name, project, **kwargs)
                print(json.dumps({"feature_id": feature_id}))

            elif action == "get":
                if len(sys.argv) < 4:
                    print("Usage: feature get <name>")
                    sys.exit(1)
                feature = get_feature(sys.argv[3])
                print(json.dumps(feature))

            elif action == "stage":
                if len(sys.argv) < 5:
                    print("Usage: feature stage <name> <stage_number>")
                    sys.exit(1)
                mark_stage_complete(sys.argv[3], int(sys.argv[4]))
                print(json.dumps({"status": "stage_marked"}))

        elif command == "file-change":
            if len(sys.argv) < 5:
                print("Usage: file-change <session_id> <path> <type> [desc] [--step=X]")
                sys.exit(1)
            session_id = sys.argv[2]
            path = sys.argv[3]
            change_type = sys.argv[4]
            desc = (
                sys.argv[5]
                if len(sys.argv) > 5 and not sys.argv[5].startswith("--")
                else None
            )

            kwargs = {}
            for arg in sys.argv[5:]:
                if arg.startswith("--"):
                    key, value = arg[2:].split("=", 1)
                    key = key.replace("-", "_")
                    if key in ["step_id", "lines_added", "lines_removed"]:
                        value = int(value)
                    kwargs[key] = value

            change_id = record_file_change(
                session_id, path, change_type, description=desc, **kwargs
            )
            print(json.dumps({"change_id": change_id}))

        elif command == "decision":
            if len(sys.argv) < 5:
                print(
                    "Usage: decision <session_id> <question> <choice> [--type=X] [--rationale=X]"
                )
                sys.exit(1)
            session_id = sys.argv[2]
            question = sys.argv[3]
            choice = sys.argv[4]

            kwargs = {}
            for arg in sys.argv[5:]:
                if arg.startswith("--"):
                    key, value = arg[2:].split("=", 1)
                    key = key.replace("-", "_")
                    if key == "type":
                        kwargs["decision_type"] = value
                    elif key == "feature":
                        kwargs["feature_name"] = value
                    elif key == "alternatives":
                        kwargs[key] = json.loads(value)
                    else:
                        kwargs[key] = value

            decision_id = record_decision(session_id, question, choice, **kwargs)
            print(json.dumps({"decision_id": decision_id}))

        elif command == "query":
            if len(sys.argv) < 3:
                print("Usage: query <sessions|steps|files|decisions|search> [args]")
                sys.exit(1)

            query_type = sys.argv[2]

            if query_type == "sessions":
                project = sys.argv[3] if len(sys.argv) > 3 else None
                days = int(sys.argv[4]) if len(sys.argv) > 4 else 7
                results = get_recent_sessions(project, days)
                print(json.dumps(results, indent=2))

            elif query_type == "steps":
                if len(sys.argv) < 4:
                    print("Usage: query steps <session_id>")
                    sys.exit(1)
                results = get_session_steps(sys.argv[3])
                print(json.dumps(results, indent=2))

            elif query_type == "files":
                days = int(sys.argv[3]) if len(sys.argv) > 3 else 7
                results = get_recent_file_changes(days)
                print(json.dumps(results, indent=2))

            elif query_type == "decisions":
                if len(sys.argv) < 4:
                    print("Usage: query decisions <feature_name>")
                    sys.exit(1)
                results = get_feature_decisions(sys.argv[3])
                print(json.dumps(results, indent=2))

            elif query_type == "search":
                if len(sys.argv) < 4:
                    print("Usage: query search <query> [type=steps|decisions]")
                    sys.exit(1)
                query_str = sys.argv[3]
                search_type = sys.argv[4] if len(sys.argv) > 4 else "steps"
                if search_type == "decisions":
                    results = search_decisions(query_str)
                else:
                    results = search_steps(query_str)
                print(json.dumps(results, indent=2))

        elif command == "summary":
            project = sys.argv[2] if len(sys.argv) > 2 else None
            days = int(sys.argv[3]) if len(sys.argv) > 3 else 30
            summary = get_journey_summary(project, days)
            print(json.dumps(summary, indent=2))

        elif command == "last-session":
            # Parse options
            project = None
            global_mode = False
            output_format = "json"

            for arg in sys.argv[2:]:
                if arg == "--global":
                    global_mode = True
                elif arg.startswith("--format="):
                    output_format = arg.split("=", 1)[1]
                elif not arg.startswith("--"):
                    project = arg

            info = get_last_session_info(project, global_mode)

            if output_format == "text":
                print(format_last_session_text(info))
            else:
                print(json.dumps(info, indent=2))

        else:
            print(f"Unknown command: {command}")
            print(__doc__)
            sys.exit(1)

    except Exception as e:
        print(json.dumps({"error": str(e)}), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()

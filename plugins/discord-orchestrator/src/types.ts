export interface ProjectConfig {
  name: string;
  path: string;
  channels: string[];
}

export interface OrchestratorConfig {
  projects: Record<string, ProjectConfig>;
  ops_channel: string;
  idle_timeout_ms: number;
}

export interface SessionState {
  sessionId: string;
  lastActiveChannel: string;
  lastActivity: string;
}

export interface SessionsState {
  [projectName: string]: SessionState;
}

export interface ActiveSession {
  projectName: string;
  sessionId: string;
  lastActiveChannel: string;
  abortController: AbortController;
  idleTimer: ReturnType<typeof setTimeout>;
  autoApproved: Set<string>;
  lastActivityAt: number;
  sendLock: Promise<void>;
}
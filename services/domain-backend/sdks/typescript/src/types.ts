// Tipos del API Domain (refleja el shape de los handlers Go).

export interface Organization {
  id: string;
  name: string;
  slug: string;
  settings: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export interface Project {
  id: string;
  organization_id: string;
  name: string;
  slug: string;
  description?: string;
  created_at: string;
}

export interface Observation {
  id: string;
  organization_id: string;
  project_id: string;
  content: string;
  observation_type: string;
  tags: string[];
  metadata: Record<string, unknown>;
  created_at: string;
}

export interface Session {
  id: string;
  title: string;
  summary?: string;
  started_at: string;
  ended_at?: string | null;
}

export type EntityType = "observation" | "prompt" | "session";

export interface SearchResult {
  entity_type: EntityType;
  id: string;
  title?: string;
  snippet?: string;
  score: number;
  project_id?: string;
  tags?: string[];
  created_at: string;
}

export interface AgentRunResult {
  run_id: string;
  status: "completed" | "failed" | "running";
  output: string;
  error?: string;
  tokens_input: number;
  tokens_output: number;
  iterations: number;
  started_at?: string;
  finished_at?: string;
}

export interface Skill {
  id: string;
  slug: string;
  name: string;
  description?: string;
  skill_type: "prompt" | "code" | "api" | "mcp_tool";
  content?: string;
  input_schema: Record<string, unknown>;
  output_schema: Record<string, unknown>;
  tags: string[];
}

export interface FlowStep {
  id: string;
  type: "agent_run" | "skill_run" | "http_request" | "mem_save" | "condition" | "parallel" | "wait_signal";
  config: Record<string, unknown>;
  on_error?: "fail" | "continue" | string;
  retries?: number;
  timeout_s?: number;
}

export interface FlowSpec {
  version: number;
  steps: FlowStep[];
}

export interface Flow {
  id: string;
  slug: string;
  name: string;
  description?: string;
  spec: FlowSpec;
  is_active: boolean;
  created_at: string;
}

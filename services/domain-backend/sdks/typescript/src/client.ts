// DomainClient — cliente fetch-based (Node 20+ tiene fetch built-in).

import {
  AuthError,
  ConflictError,
  DomainError,
  NotFoundError,
  QuotaExceededError,
  RateLimitError,
  ValidationError,
} from "./errors.js";
import type {
  AgentRunResult,
  Flow,
  FlowSpec,
  Observation,
  Organization,
  Project,
  SearchResult,
  Session,
  Skill,
} from "./types.js";

export interface DomainClientOptions {
  apiKey?: string;
  baseUrl?: string;
  fetchImpl?: typeof fetch;
  timeoutMs?: number;
}

export class DomainClient {
  private readonly apiKey: string;
  private readonly baseUrl: string;
  private readonly fetch: typeof fetch;
  private readonly timeoutMs: number;

  // Resources
  readonly organizations: OrganizationsResource;
  readonly projects: ProjectsResource;
  readonly observations: ObservationsResource;
  readonly sessions: SessionsResource;
  readonly search: SearchResource;
  readonly skills: SkillsResource;
  readonly agents: AgentsResource;
  readonly flows: FlowsResource;
  readonly knowledge: KnowledgeResource;

  constructor(opts: DomainClientOptions = {}) {
    const apiKey = opts.apiKey ?? process.env.DOMAIN_API_KEY ?? "";
    if (!apiKey) {
      throw new AuthError({
        message: "api key required (pass arg or set DOMAIN_API_KEY env)",
      });
    }
    this.apiKey = apiKey;
    this.baseUrl = (opts.baseUrl ?? process.env.DOMAIN_BASE_URL ?? "http://localhost:8000")
      .replace(/\/+$/, "");
    this.fetch = opts.fetchImpl ?? globalThis.fetch.bind(globalThis);
    this.timeoutMs = opts.timeoutMs ?? 30_000;

    this.organizations = new OrganizationsResource(this);
    this.projects = new ProjectsResource(this);
    this.observations = new ObservationsResource(this);
    this.sessions = new SessionsResource(this);
    this.search = new SearchResource(this);
    this.skills = new SkillsResource(this);
    this.agents = new AgentsResource(this);
    this.flows = new FlowsResource(this);
    this.knowledge = new KnowledgeResource(this);
  }

  async request<T = unknown>(
    method: string,
    path: string,
    opts: { body?: unknown; query?: Record<string, string | number | undefined> } = {},
  ): Promise<T> {
    const url = new URL(`${this.baseUrl}/api/v1${path}`);
    if (opts.query) {
      for (const [k, v] of Object.entries(opts.query)) {
        if (v !== undefined && v !== "") url.searchParams.set(k, String(v));
      }
    }
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), this.timeoutMs);
    try {
      const resp = await this.fetch(url.toString(), {
        method,
        headers: {
          Authorization: `Bearer ${this.apiKey}`,
          "Content-Type": "application/json",
          "User-Agent": "@domain/sdk-ts/0.1.0",
        },
        body: opts.body ? JSON.stringify(opts.body) : undefined,
        signal: controller.signal,
      });
      if (resp.status === 204) return undefined as T;
      const raw = await resp.text();
      const body = raw ? JSON.parse(raw) : {};
      if (resp.ok) {
        return (body.data ?? body) as T;
      }
      throw translateError(resp, body);
    } finally {
      clearTimeout(timer);
    }
  }
}

function translateError(
  resp: Response,
  body: { error?: { code?: string; message?: string; request_id?: string; details?: unknown[] } },
): DomainError {
  const err = body.error ?? {};
  const opts = {
    message: err.message ?? resp.statusText,
    code: err.code ?? "",
    status: resp.status,
    requestId: err.request_id ?? resp.headers.get("X-Request-Id") ?? "",
    details: (err.details ?? []) as never[],
  };
  switch (resp.status) {
    case 401:
    case 403:
      return new AuthError(opts);
    case 402:
      return new QuotaExceededError(opts);
    case 404:
      return new NotFoundError(opts);
    case 409:
      return new ConflictError(opts);
    case 422:
      return new ValidationError(opts);
    case 429:
      return new RateLimitError({
        ...opts,
        retryAfter: Number(resp.headers.get("Retry-After") ?? 0),
      });
    default:
      return new DomainError(opts);
  }
}

// ===== Resources =====

class OrganizationsResource {
  constructor(private c: DomainClient) {}
  create(input: { name: string; slug: string; owner_email: string; owner_name?: string }) {
    return this.c.request<{ organization: Organization }>("POST", "/organizations", { body: input });
  }
  get(id: string) {
    return this.c.request<Organization>("GET", `/organizations/${id}`);
  }
  listMembers(orgId: string) {
    return this.c.request<Array<Record<string, unknown>>>("GET", `/organizations/${orgId}/members`);
  }
}

class ProjectsResource {
  constructor(private c: DomainClient) {}
  create(input: { name: string; slug: string; description?: string }) {
    return this.c.request<Project>("POST", "/projects", { body: input });
  }
  list() {
    return this.c.request<Project[]>("GET", "/projects");
  }
  get(slug: string) {
    return this.c.request<Project>("GET", `/projects/${slug}`);
  }
  delete(slug: string) {
    return this.c.request<void>("DELETE", `/projects/${slug}`);
  }
}

class ObservationsResource {
  constructor(private c: DomainClient) {}
  save(input: {
    project_slug: string;
    content: string;
    observation_type?: string;
    tags?: string[];
    metadata?: Record<string, unknown>;
  }) {
    return this.c.request<Observation>("POST", "/observations", { body: input });
  }
  get(id: string) {
    return this.c.request<Observation>("GET", `/observations/${id}`);
  }
  list(opts: { project_slug: string; limit?: number }) {
    return this.c.request<Observation[]>("GET", "/observations", {
      query: { project_slug: opts.project_slug, limit: opts.limit },
    });
  }
  delete(id: string) {
    return this.c.request<void>("DELETE", `/observations/${id}`);
  }
}

class SessionsResource {
  constructor(private c: DomainClient) {}
  start(input: { title: string; project_slug?: string; tags?: string[] }) {
    return this.c.request<Session>("POST", "/sessions", { body: input });
  }
  end(sessionId: string, opts: { summary?: string } = {}) {
    return this.c.request<Session>("POST", `/sessions/${sessionId}/end`, { body: opts });
  }
  active(opts: { project_slug?: string } = {}) {
    return this.c.request<Session | null>("GET", "/sessions/active", {
      query: { project_slug: opts.project_slug },
    });
  }
}

class SearchResource {
  constructor(private c: DomainClient) {}
  global(opts: {
    query: string;
    limit?: number;
    entityTypes?: string[];
    tags?: string[];
  }) {
    return this.c.request<SearchResult[]>("GET", "/search", {
      query: {
        q: opts.query,
        limit: opts.limit,
        entity_type: opts.entityTypes?.join(","),
        tags: opts.tags?.join(","),
      },
    });
  }
}

class SkillsResource {
  constructor(private c: DomainClient) {}
  list(opts: { type?: string; tag?: string; limit?: number } = {}) {
    return this.c.request<Skill[]>("GET", "/skills", { query: opts });
  }
  create(input: Record<string, unknown>) {
    return this.c.request<Skill>("POST", "/skills", { body: input });
  }
}

class AgentsResource {
  constructor(private c: DomainClient) {}
  list() {
    return this.c.request<Array<Record<string, unknown>>>("GET", "/agents");
  }
  get(id: string) {
    return this.c.request<Record<string, unknown>>("GET", `/agents/${id}`);
  }
  run(agentId: string, input: string, variables?: Record<string, unknown>) {
    return this.c.request<AgentRunResult>("POST", `/agents/${agentId}/run`, {
      body: { input, variables: variables ?? {} },
    });
  }
  runLogs(runId: string) {
    return this.c.request<Array<Record<string, unknown>>>("GET", `/agent-runs/${runId}/logs`);
  }
}

class FlowsResource {
  constructor(private c: DomainClient) {}
  list() {
    return this.c.request<Flow[]>("GET", "/flows");
  }
  create(input: { slug: string; name: string; description?: string; spec: FlowSpec }) {
    return this.c.request<Flow>("POST", "/flows", { body: input });
  }
  run(flowId: string, inputs: Record<string, unknown> = {}) {
    return this.c.request<Record<string, unknown>>("POST", `/flows/${flowId}/run`, {
      body: { inputs },
    });
  }
}

class KnowledgeResource {
  constructor(private c: DomainClient) {}
  save(input: {
    project_slug: string;
    title: string;
    body: string;
    source?: string;
    tags?: string[];
  }) {
    return this.c.request<{ document: Record<string, unknown>; chunks_count: number }>(
      "POST",
      "/knowledge",
      { body: input },
    );
  }
  search(query: string, limit = 20) {
    return this.c.request<Array<Record<string, unknown>>>("GET", "/knowledge/search", {
      query: { q: query, limit },
    });
  }
}

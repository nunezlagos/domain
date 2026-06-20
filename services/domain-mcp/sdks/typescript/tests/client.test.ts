// Tests E2E del SDK TS contra mock fetch in-process.
// No usa MSW para evitar overhead; mock fetch directo basta.

import { describe, expect, it, beforeEach, vi } from "vitest";
import {
  AuthError,
  ConflictError,
  DomainClient,
  NotFoundError,
  QuotaExceededError,
  ValidationError,
} from "../src/index.js";

function makeClient(fetchImpl: typeof fetch) {
  return new DomainClient({
    apiKey: "domk_test_xxxx",
    baseUrl: "http://test.local",
    fetchImpl,
  });
}

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

describe("DomainClient", () => {
  it("creates project — happy path", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      jsonResponse(201, {
        data: {
          id: "11111111-1111-1111-1111-111111111111",
          name: "Demo",
          slug: "demo",
          description: "",
          created_at: "2026-06-07T00:00:00Z",
        },
      }),
    );
    const c = makeClient(fetchImpl as unknown as typeof fetch);
    const proj = await c.projects.create({ name: "Demo", slug: "demo" });
    expect(proj.slug).toBe("demo");
    expect(fetchImpl).toHaveBeenCalledWith(
      "http://test.local/api/v1/projects",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer domk_test_xxxx",
        }),
      }),
    );
  });

  it("translates 401 to AuthError", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      jsonResponse(401, { error: { code: "unauthorized", message: "unauthorized" } }),
    );
    const c = makeClient(fetchImpl as unknown as typeof fetch);
    await expect(c.projects.create({ name: "X", slug: "x" })).rejects.toBeInstanceOf(AuthError);
  });

  it("translates 404 to NotFoundError", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      jsonResponse(404, { error: { code: "not_found", message: "project not found" } }),
    );
    const c = makeClient(fetchImpl as unknown as typeof fetch);
    await expect(c.projects.get("no-existe")).rejects.toBeInstanceOf(NotFoundError);
  });

  it("translates 409 to ConflictError preserving code", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      jsonResponse(409, { error: { code: "slug_taken", message: "slug already exists" } }),
    );
    const c = makeClient(fetchImpl as unknown as typeof fetch);
    try {
      await c.projects.create({ name: "X", slug: "dup" });
      expect.fail("must throw");
    } catch (e) {
      expect(e).toBeInstanceOf(ConflictError);
      expect((e as ConflictError).code).toBe("slug_taken");
    }
  });

  it("translates 422 to ValidationError", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      jsonResponse(422, {
        error: {
          code: "validation_failed",
          message: "validation failed",
          details: [{ field: "slug", code: "invalid_format", message: "lowercase" }],
        },
      }),
    );
    const c = makeClient(fetchImpl as unknown as typeof fetch);
    try {
      await c.projects.create({ name: "X", slug: "UPPER" });
      expect.fail("must throw");
    } catch (e) {
      expect(e).toBeInstanceOf(ValidationError);
      expect((e as ValidationError).details).toHaveLength(1);
    }
  });

  it("translates 402 to QuotaExceededError", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      jsonResponse(402, { error: { code: "quota_exceeded", message: "tokens exhausted" } }),
    );
    const c = makeClient(fetchImpl as unknown as typeof fetch);
    await expect(c.agents.run("aaa", "hi")).rejects.toBeInstanceOf(QuotaExceededError);
  });

  it("requires api_key", () => {
    expect(() => new DomainClient({ apiKey: "", baseUrl: "http://x" })).toThrow(AuthError);
  });

  it("observations.save sends correct payload", async () => {
    const fetchImpl = vi.fn().mockResolvedValue(
      jsonResponse(201, {
        data: {
          id: "33333333-3333-3333-3333-333333333333",
          project_id: "11111111-1111-1111-1111-111111111111",
          content: "hola",
          observation_type: "note",
          tags: [],
          metadata: {},
          created_at: "2026-06-07T00:00:00Z",
        },
      }),
    );
    const c = makeClient(fetchImpl as unknown as typeof fetch);
    const obs = await c.observations.save({
      project_slug: "demo",
      content: "hola",
    });
    expect(obs.content).toBe("hola");
    const callArgs = (fetchImpl as unknown as ReturnType<typeof vi.fn>).mock.calls[0]?.[1] as {
      body?: string;
    };
    const sentBody = JSON.parse(callArgs?.body ?? "{}");
    expect(sentBody.project_slug).toBe("demo");
  });
});

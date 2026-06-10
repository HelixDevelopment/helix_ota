// Helix OTA dashboard — unit tests for the typed /api/v1 fetch client.
// Pure client logic: URL + query building, bearer attach, the single
// transparent 401 -> refresh -> retry path, and ApiErrorEnvelope -> ApiError
// mapping. A fetch stub is used — permitted ONLY in unit tests (§11.4.27).

import { describe, it, expect, beforeEach, afterEach, vi } from "vitest";
import { ApiClient, ApiError, type TokenBridge } from "./client";

// A captured fetch call (url + init) so assertions can inspect what was sent.
interface Call {
  url: string;
  init: RequestInit;
}

function jsonResponse(status: number, body: unknown, headers: Record<string, string> = {}): Response {
  return new Response(status === 204 ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...headers },
  });
}

describe("ApiClient.buildUrl + query", () => {
  let calls: Call[];

  beforeEach(() => {
    calls = [];
    vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return Promise.resolve(jsonResponse(200, { items: [] }));
    });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("builds the base path against the configured base url", async () => {
    const client = new ApiClient("http://api.test/api/v1");
    await client.listReleases();
    expect(calls[0].url).toBe("http://api.test/api/v1/releases");
  });

  it("appends defined query params and skips undefined ones", async () => {
    const client = new ApiClient("http://api.test/api/v1");
    await client.listReleases({ os: "android", limit: 5, cursor: undefined });
    const u = new URL(calls[0].url);
    expect(u.pathname).toBe("/api/v1/releases");
    expect(u.searchParams.get("os")).toBe("android");
    expect(u.searchParams.get("limit")).toBe("5");
    expect(u.searchParams.has("cursor")).toBe(false);
  });

  it("percent-encodes path identifiers", async () => {
    const client = new ApiClient("http://api.test/api/v1");
    await client.getRelease("rel/with space");
    expect(calls[0].url).toBe("http://api.test/api/v1/releases/rel%2Fwith%20space");
  });

  it("joins base + path verbatim (an explicit base is used as-given)", async () => {
    // The constructor takes the base url verbatim; only the resolved DEFAULT base
    // (resolveBaseUrl) strips a trailing slash. An explicit base passed by a caller
    // is its own responsibility — document that real behaviour here.
    const client = new ApiClient("http://api.test/api/v1");
    await client.getTelemetryOverview();
    expect(calls[0].url).toBe("http://api.test/api/v1/telemetry/overview");
  });
});

describe("ApiClient bearer + anonymous", () => {
  let calls: Call[];

  beforeEach(() => {
    calls = [];
    vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return Promise.resolve(jsonResponse(200, {}));
    });
  });

  afterEach(() => vi.unstubAllGlobals());

  function headerOf(init: RequestInit, name: string): string | null {
    return (init.headers as Record<string, string>)[name] ?? null;
  }

  it("attaches Authorization: Bearer <access> when a token is present", async () => {
    const client = new ApiClient("http://api.test/api/v1");
    const bridge: TokenBridge = {
      getAccessToken: () => "tok-123",
      refresh: () => Promise.resolve(null),
      onSessionExpired: () => {},
    };
    client.attachTokenBridge(bridge);
    await client.getTelemetryOverview();
    expect(headerOf(calls[0].init, "Authorization")).toBe("Bearer tok-123");
    expect(headerOf(calls[0].init, "Accept")).toBe("application/json");
    expect(headerOf(calls[0].init, "Accept-Encoding")).toBe("br, gzip");
  });

  it("omits Authorization on anonymous routes (login)", async () => {
    const client = new ApiClient("http://api.test/api/v1");
    const bridge: TokenBridge = {
      getAccessToken: () => "tok-123",
      refresh: () => Promise.resolve(null),
      onSessionExpired: () => {},
    };
    client.attachTokenBridge(bridge);
    await client.login({ username: "a@b.c", password: "pw" });
    expect(headerOf(calls[0].init, "Authorization")).toBeNull();
    expect(headerOf(calls[0].init, "Content-Type")).toBe("application/json");
    expect(calls[0].init.body).toBe(JSON.stringify({ username: "a@b.c", password: "pw" }));
  });
});

describe("ApiClient 401 -> refresh -> retry", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("refreshes once on 401 and retries with the rotated token", async () => {
    const calls: Call[] = [];
    let firstAccess = "stale";
    vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
      calls.push({ url, init });
      const auth = (init.headers as Record<string, string>)["Authorization"];
      if (auth === "Bearer stale") return Promise.resolve(jsonResponse(401, {}));
      return Promise.resolve(jsonResponse(200, { event_counts: {}, total: 0, failure_rate: 0, by_state: {} }));
    });

    const refresh = vi.fn(() => {
      firstAccess = "fresh";
      return Promise.resolve("fresh");
    });
    const onSessionExpired = vi.fn();
    const client = new ApiClient("http://api.test/api/v1");
    client.attachTokenBridge({
      getAccessToken: () => firstAccess,
      refresh,
      onSessionExpired,
    });

    const result = await client.getTelemetryOverview();
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(onSessionExpired).not.toHaveBeenCalled();
    // Two fetches: the 401 attempt + the retried attempt with the fresh token.
    expect(calls).toHaveLength(2);
    expect((calls[1].init.headers as Record<string, string>)["Authorization"]).toBe("Bearer fresh");
    expect(result.failure_rate).toBe(0);
  });

  it("does NOT retry a second time and calls onSessionExpired when refresh fails", async () => {
    const calls: Call[] = [];
    vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return Promise.resolve(jsonResponse(401, {}));
    });
    const refresh = vi.fn(() => Promise.resolve(null));
    const onSessionExpired = vi.fn();
    const client = new ApiClient("http://api.test/api/v1");
    client.attachTokenBridge({
      getAccessToken: () => "dead",
      refresh,
      onSessionExpired,
    });

    await expect(client.getTelemetryOverview()).rejects.toBeInstanceOf(ApiError);
    expect(refresh).toHaveBeenCalledTimes(1);
    expect(onSessionExpired).toHaveBeenCalledTimes(1);
    // Only ONE network attempt — no retry when refresh yields no token.
    expect(calls).toHaveLength(1);
  });

  it("does not attempt refresh on the refresh route itself (noRefresh)", async () => {
    const calls: Call[] = [];
    vi.stubGlobal("fetch", (url: string, init: RequestInit) => {
      calls.push({ url, init });
      return Promise.resolve(jsonResponse(401, {}));
    });
    const refresh = vi.fn(() => Promise.resolve("x"));
    const client = new ApiClient("http://api.test/api/v1");
    client.attachTokenBridge({
      getAccessToken: () => "tok",
      refresh,
      onSessionExpired: () => {},
    });
    await expect(client.refresh("rt-1")).rejects.toBeInstanceOf(ApiError);
    expect(refresh).not.toHaveBeenCalled();
    expect(calls).toHaveLength(1);
  });
});

describe("ApiClient error mapping", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("maps the server error envelope into a typed ApiError", async () => {
    vi.stubGlobal("fetch", () =>
      Promise.resolve(
        jsonResponse(
          409,
          {
            error: {
              code: "VERSION_NOT_MONOTONIC",
              message: "version must increase",
              request_id: "req-9",
              details: [{ field: "version", message: "too low" }],
            },
          },
          { "X-Request-Id": "req-header-ignored" },
        ),
      ),
    );
    const client = new ApiClient("http://api.test/api/v1");
    client.attachTokenBridge({
      getAccessToken: () => "t",
      refresh: () => Promise.resolve(null),
      onSessionExpired: () => {},
    });

    const err = (await client.createRelease({
      artifact_id: "art",
      version: "0.0.1",
      os: "android",
      target_model: "rk3588",
    }).catch((e) => e)) as ApiError;

    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(409);
    expect(err.code).toBe("VERSION_NOT_MONOTONIC");
    expect(err.message).toBe("version must increase");
    expect(err.requestId).toBe("req-9");
    expect(err.details).toEqual([{ field: "version", message: "too low" }]);
  });

  it("falls back to the X-Request-Id header and status text on a non-JSON error body", async () => {
    vi.stubGlobal("fetch", () =>
      Promise.resolve(
        new Response("<html>502</html>", {
          status: 502,
          statusText: "Bad Gateway",
          headers: { "Content-Type": "text/html", "X-Request-Id": "req-html" },
        }),
      ),
    );
    const client = new ApiClient("http://api.test/api/v1");
    client.attachTokenBridge({
      getAccessToken: () => "t",
      refresh: () => Promise.resolve(null),
      onSessionExpired: () => {},
    });

    const err = (await client.getTelemetryOverview().catch((e) => e)) as ApiError;
    expect(err).toBeInstanceOf(ApiError);
    expect(err.status).toBe(502);
    expect(err.code).toBe("UNKNOWN");
    expect(err.message).toBe("Bad Gateway");
    expect(err.requestId).toBe("req-html");
    expect(err.details).toEqual([]);
  });

  it("returns undefined for a 204 No Content response", async () => {
    vi.stubGlobal("fetch", () => Promise.resolve(jsonResponse(204, null)));
    const client = new ApiClient("http://api.test/api/v1");
    client.attachTokenBridge({
      getAccessToken: () => "t",
      refresh: () => Promise.resolve(null),
      onSessionExpired: () => {},
    });
    const out = await client.getTelemetryOverview();
    expect(out).toBeUndefined();
  });
});

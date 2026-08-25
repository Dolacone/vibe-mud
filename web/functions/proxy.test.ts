import { afterEach, describe, expect, it, vi } from "vitest";
import { onRequest } from "./[[path]]";

const backendOrigin = "https://vibe-mud-api.fly.dev";

function context(path: string, init: RequestInit = {}, origin = backendOrigin) {
  const request = new Request(`https://vibe-mud.pages.dev${path}`, init);
  if (/%2e/i.test(path)) {
    Object.defineProperty(request, "url", {
      value: `https://vibe-mud.pages.dev${path}`,
    });
  }
  return {
    request,
    env: { BACKEND_ORIGIN: origin },
    functionPath: "",
    params: {},
    data: {},
    waitUntil: () => undefined,
    passThroughOnException: () => undefined,
    next: vi.fn(async () => new Response("static asset", { status: 200 })),
  } as unknown as Parameters<typeof onRequest>[0];
}

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("Pages proxy", () => {
  it("forwards an allow-listed query and cookie with manual redirects", async () => {
    const upstream = new Response("redirect", {
      status: 302,
      headers: {
        Location: "https://accounts.google.com/o/oauth2/v2/auth?state=opaque",
        "X-Upstream": "kept",
      },
    });
    const fetcher = vi.fn((..._args: Parameters<typeof fetch>) => Promise.resolve(upstream));
    vi.stubGlobal("fetch", fetcher);
    const log = vi.spyOn(console, "log");

    const response = await onRequest(
      context("/auth/google/callback?state=opaque&code=secret-code", {
        headers: { Cookie: "mud_oauth_flow=secret-cookie" },
      }),
    );

    expect(response.status).toBe(302);
    expect(response.headers.get("location")).toContain("accounts.google.com");
    expect(response.headers.get("x-upstream")).toBe("kept");
    expect(fetcher).toHaveBeenCalledOnce();
    const call = fetcher.mock.calls[0];
    expect(call).toBeDefined();
    const [request, options] = call!;
    expect(String(request)).toBe(
      "https://vibe-mud-api.fly.dev/auth/google/callback?state=opaque&code=secret-code",
    );
    expect(options?.method).toBe("GET");
    expect(options?.redirect).toBe("manual");
    expect(options?.headers).toBeInstanceOf(Headers);
    expect(options?.headers instanceof Headers ? options.headers.get("cookie") : null).toBe(
      "mud_oauth_flow=secret-cookie",
    );
    expect(log).not.toHaveBeenCalled();
  });

  it("preserves each upstream Set-Cookie header", async () => {
    const headers = new Headers({ Location: "https://vibe-mud.pages.dev/" });
    headers.append("Set-Cookie", "mud_oauth_flow=; Max-Age=0; Path=/");
    headers.append("Set-Cookie", "mud_session=session-token; Path=/; HttpOnly");
    const fetcher = vi.fn(async () => new Response(null, { status: 302, headers }));
    vi.stubGlobal("fetch", fetcher);

    const response = await onRequest(context("/auth/google/callback?state=s&code=c"));

    expect(response.headers.getSetCookie()).toEqual([
      "mud_oauth_flow=; Max-Age=0; Path=/",
      "mud_session=session-token; Path=/; HttpOnly",
    ]);
  });

  it("allows OPTIONS to reach the backend", async () => {
    const fetcher = vi.fn((..._args: Parameters<typeof fetch>) =>
      Promise.resolve(new Response(null, { status: 204 })));
    vi.stubGlobal("fetch", fetcher);

    const response = await onRequest(context("/api/me", { method: "OPTIONS" }));

    expect(response.status).toBe(204);
    expect(fetcher).toHaveBeenCalledOnce();
    expect(fetcher.mock.calls[0]?.[1]?.method).toBe("OPTIONS");
  });

  it("forwards the same-origin rest POST and preserves the browser Origin", async () => {
    const fetcher = vi.fn((..._args: Parameters<typeof fetch>) =>
      Promise.resolve(new Response(JSON.stringify({ ap: 2999 }), {
        status: 200,
        headers: { "content-type": "application/json" },
      })));
    vi.stubGlobal("fetch", fetcher);

    const response = await onRequest(context("/api/actions/rest", {
      method: "POST",
      headers: {
        Cookie: "mud_session=session-token",
        Origin: "https://vibe-mud.pages.dev",
      },
    }));

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ ap: 2999 });
    expect(fetcher).toHaveBeenCalledOnce();
    const [, options] = fetcher.mock.calls[0]!;
    expect(options?.method).toBe("POST");
    expect(options?.headers).toBeInstanceOf(Headers);
    expect(options?.headers instanceof Headers ? options.headers.get("cookie") : null).toBe(
      "mud_session=session-token",
    );
    expect(options?.headers instanceof Headers ? options.headers.get("origin") : null).toBe(
      "https://vibe-mud.pages.dev",
    );
  });

  it("blocks methods outside the proxy contract", async () => {
    const fetcher = vi.fn();
    vi.stubGlobal("fetch", fetcher);

    const response = await onRequest(context("/api/me", { method: "POST" }));

    expect(response.status).toBe(405);
    expect(await response.json()).toEqual({ error: "method not allowed" });
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("passes unrelated paths to Pages static assets", async () => {
    const fetcher = vi.fn();
    vi.stubGlobal("fetch", fetcher);
    const requestContext = context("/assets/app.js");

    const response = await onRequest(requestContext);

    expect(response.status).toBe(200);
    expect(await response.text()).toBe("static asset");
    expect(requestContext.next).toHaveBeenCalledOnce();
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("passes traversal and malformed encoded paths to static assets", async () => {
    const fetcher = vi.fn((input: RequestInfo | URL) => {
      throw new Error(`unexpected proxy: ${String(input)}`);
    });
    vi.stubGlobal("fetch", fetcher);

    for (const path of [
      "/api/%2e%2e/auth/google/login",
      "/api/..%2fauth/google/login",
      "/api/%252e%252e/auth/google/login",
      "/api/%2fsecret",
      "/api/%E0%A4%A",
    ]) {
      const requestContext = context(path);
      const response = await onRequest(requestContext);
      expect(response.status, path).toBe(200);
      expect(requestContext.next, path).toHaveBeenCalledOnce();
    }
    expect(fetcher).not.toHaveBeenCalled();
  });

  it("rejects a backend value that is not an origin-only HTTPS URL", async () => {
    const fetcher = vi.fn();
    vi.stubGlobal("fetch", fetcher);
    const invalidOrigins = [
      "http://vibe-mud-api.fly.dev",
      "https://user:password@vibe-mud-api.fly.dev",
      "https://vibe-mud-api.fly.dev/private",
      "https://vibe-mud-api.fly.dev?token=secret",
      "https://vibe-mud-api.fly.dev#fragment",
    ];

    for (const origin of invalidOrigins) {
      const response = await onRequest(context("/api/me", {}, origin));
      expect(response.status, origin).toBe(500);
      expect(await response.json(), origin).toEqual({ error: "proxy unavailable" });
    }
    expect(fetcher).not.toHaveBeenCalled();
  });
});

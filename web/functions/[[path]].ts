import type { PagesFunction } from "@cloudflare/workers-types";

interface Env {
  BACKEND_ORIGIN?: string;
}

const allowedPathPrefixes = ["/auth/", "/api/"];
type PagesResponse = Awaited<ReturnType<PagesFunction<Env>>>;

function pagesResponse(response: Response): PagesResponse {
  return response as unknown as PagesResponse;
}

function configuredBackendOrigin(value: string | undefined): URL | null {
  if (!value) return null;

  let origin: URL;
  try {
    origin = new URL(value);
  } catch {
    return null;
  }

  if (
    origin.protocol !== "https:" ||
    origin.username ||
    origin.password ||
    (origin.pathname !== "" && origin.pathname !== "/") ||
    origin.search ||
    origin.hash ||
    !origin.hostname
  ) {
    return null;
  }
  return origin;
}

function proxyPath(rawPath: string): string | null {
  if (/%(?:2e|2f|5c|25)/i.test(rawPath)) return null;

  let decodedPath: string;
  try {
    decodedPath = decodeURIComponent(rawPath);
  } catch {
    return null;
  }
  if (
    decodedPath.includes("\\") ||
    decodedPath.includes("\u0000") ||
    decodedPath.includes("?") ||
    decodedPath.includes("#")
  ) {
    return null;
  }

  const segments = decodedPath.split("/");
  if (segments.some((segment) => segment === "." || segment === "..")) return null;

  const normalized = new URL(`https://pages.invalid${decodedPath}`).pathname;
  if (!allowedPathPrefixes.some((prefix) => normalized.startsWith(prefix))) return null;
  return normalized;
}

function copyResponseHeaders(upstream: Response): Headers {
  const headers = new Headers(upstream.headers);
  const cookies = upstream.headers.getSetCookie();
  headers.delete("set-cookie");
  for (const cookie of cookies) headers.append("set-cookie", cookie);
  return headers;
}

function methodNotAllowed(): PagesResponse {
  return pagesResponse(new Response(JSON.stringify({ error: "method not allowed" }), {
    status: 405,
    headers: { "content-type": "application/json" },
  }));
}

export const onRequest: PagesFunction<Env> = async (context) => {
  const requestURLText = context.request.url;
  const requestURL = new URL(requestURLText);
  const rawPath = requestURLText.match(
    /^[a-z][a-z\d+.-]*:\/\/[^\/?#]*(\/[^?#]*)?(?:[?#]|$)/i,
  )?.[1] ?? "/";
  const path = proxyPath(rawPath);
  if (!path) return context.next();
  if (context.request.method !== "GET" && context.request.method !== "OPTIONS") {
    return methodNotAllowed();
  }

  const backendOrigin = configuredBackendOrigin(context.env.BACKEND_ORIGIN);
  if (!backendOrigin) {
    return pagesResponse(new Response(JSON.stringify({ error: "proxy unavailable" }), {
      status: 500,
      headers: { "content-type": "application/json" },
    }));
  }

  const upstreamURL = new URL(backendOrigin.origin);
  upstreamURL.pathname = path;
  upstreamURL.search = requestURL.search;
  const requestHeaders = new Headers();
  const cookie = context.request.headers.get("cookie");
  if (cookie) requestHeaders.set("cookie", cookie);
  const accept = context.request.headers.get("accept");
  if (accept) requestHeaders.set("accept", accept);
  const upstream = await fetch(upstreamURL, {
    method: context.request.method,
    headers: requestHeaders,
    redirect: "manual",
  });

  return pagesResponse(new Response(upstream.body, {
    status: upstream.status,
    statusText: upstream.statusText,
    headers: copyResponseHeaders(upstream),
  }));
};

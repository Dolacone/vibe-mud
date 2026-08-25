export type CurrentUser = {
  id: number;
  display_name: string;
  email: string;
  ap: number;
};

export type AuthResult =
  | { status: "authenticated"; user: CurrentUser }
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

export type RestResult =
  | { status: "success"; ap: number }
  | { status: "insufficient"; error: string; ap: number }
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

const maxAP = 3000;

function isAP(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 0 && value <= maxAP;
}

function isCurrentUser(value: unknown): value is CurrentUser {
  if (typeof value !== "object" || value === null) return false;
  const user = value as Record<string, unknown>;
  return (
    typeof user.id === "number" &&
    typeof user.display_name === "string" &&
    typeof user.email === "string" &&
    isAP(user.ap)
  );
}

function isRestResponse(value: unknown): value is { ap: number } {
  return typeof value === "object" && value !== null && isAP((value as Record<string, unknown>).ap);
}

function isRestConflict(value: unknown): value is { error: string; ap: number } {
  if (typeof value !== "object" || value === null) return false;
  const body = value as Record<string, unknown>;
  return typeof body.error === "string" && isAP(body.ap);
}

export async function getCurrentUser(
  fetcher: typeof fetch = fetch,
): Promise<AuthResult> {
  try {
    const response = await fetcher("/api/me", {
      method: "GET",
      credentials: "include",
      headers: { Accept: "application/json" },
    });

    if (response.status === 401) return { status: "unauthenticated" };
    if (!response.ok) {
      return {
        status: "error",
        error: new Error(`identity request failed with status ${response.status}`),
      };
    }

    const body: unknown = await response.json();
    if (!isCurrentUser(body)) {
      return { status: "error", error: new Error("identity response is invalid") };
    }
    return {
      status: "authenticated",
      user: {
        id: body.id,
        display_name: body.display_name,
        email: body.email,
        ap: body.ap,
      },
    };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error("identity request failed"),
    };
  }
}

export async function rest(fetcher: typeof fetch = fetch): Promise<RestResult> {
  try {
    const response = await fetcher("/api/actions/rest", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json" },
    });

    if (response.status === 401) return { status: "unauthenticated" };

    if (response.status === 409) {
      const body: unknown = await response.json();
      if (!isRestConflict(body)) {
        return { status: "error", error: new Error("rest response is invalid") };
      }
      return { status: "insufficient", error: body.error, ap: body.ap };
    }

    if (response.status !== 200) {
      return {
        status: "error",
        error: new Error(`rest request failed with status ${response.status}`),
      };
    }

    const body: unknown = await response.json();
    if (!isRestResponse(body)) {
      return { status: "error", error: new Error("rest response is invalid") };
    }
    return { status: "success", ap: body.ap };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error("rest request failed"),
    };
  }
}

export type Location = {
  id: string;
  display_name: string;
};

export type Route = {
  origin_id: string;
  destination_id: string;
  ap_cost: number;
};

export type PlayerState = {
  location: Location;
  routes: Route[];
  ap: number;
};

export type CurrentUser = PlayerState & {
  id: number;
  display_name: string;
  email: string;
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

export type MoveResult =
  | ({ status: "success" } & PlayerState)
  | ({ status: "insufficient"; error: string } & PlayerState)
  | ({ status: "invalid"; error: string; state?: PlayerState })
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

const maxAP = 3000;

function isAP(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 0 && value <= maxAP;
}

function isString(value: unknown): value is string {
  return typeof value === "string" && value.trim() !== "";
}

function isLocation(value: unknown): value is Location {
  if (typeof value !== "object" || value === null) return false;
  const location = value as Record<string, unknown>;
  return isString(location.id) && isString(location.display_name);
}

function isRoute(value: unknown): value is Route {
  if (typeof value !== "object" || value === null) return false;
  const route = value as Record<string, unknown>;
  return (
    isString(route.origin_id) &&
    isString(route.destination_id) &&
    typeof route.ap_cost === "number" &&
    Number.isInteger(route.ap_cost) &&
    route.ap_cost > 0
  );
}

function isPlayerState(value: unknown): value is PlayerState {
  if (typeof value !== "object" || value === null) return false;
  const state = value as Record<string, unknown>;
  const location = state.location;
  const routes = state.routes;
  return (
    isLocation(location) &&
    Array.isArray(routes) &&
    routes.every(isRoute) &&
    routes.every((route) => route.origin_id === location.id) &&
    isAP(state.ap)
  );
}

function isCurrentUser(value: unknown): value is CurrentUser {
  if (typeof value !== "object" || value === null) return false;
  const user = value as Record<string, unknown>;
  return (
    typeof user.id === "number" &&
    typeof user.display_name === "string" &&
    typeof user.email === "string" &&
    isPlayerState(user)
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

function isMoveError(value: unknown): value is { error: string } {
  return typeof value === "object" && value !== null && typeof (value as Record<string, unknown>).error === "string";
}

function isMoveStateResponse(value: unknown): value is PlayerState {
  return isPlayerState(value);
}

function isMoveConflict(value: unknown): value is { error: string } & PlayerState {
  return isMoveError(value) && isMoveStateResponse(value);
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
        location: body.location,
        routes: body.routes,
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

export async function move(target: string, fetcher: typeof fetch = fetch): Promise<MoveResult> {
  if (!isString(target)) {
    return { status: "invalid", error: "invalid target" };
  }

  try {
    const response = await fetcher("/api/actions/move", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify({ target }),
    });

    if (response.status === 401) return { status: "unauthenticated" };

    if (response.status === 409) {
      const body: unknown = await response.json();
      if (!isMoveConflict(body)) {
        return { status: "error", error: new Error("move response is invalid") };
      }
      return {
        status: "insufficient",
        error: body.error,
        location: body.location,
        routes: body.routes,
        ap: body.ap,
      };
    }

    if (response.status === 400) {
      const body: unknown = await response.json();
      if (!isMoveError(body)) {
        return { status: "error", error: new Error("move response is invalid") };
      }
      if (isMoveStateResponse(body)) {
        const { error, ...state } = body;
        return { status: "invalid", error, state };
      }
      return { status: "invalid", error: body.error };
    }

    if (response.status !== 200) {
      return {
        status: "error",
        error: new Error(`move request failed with status ${response.status}`),
      };
    }

    const body: unknown = await response.json();
    if (!isMoveStateResponse(body)) {
      return { status: "error", error: new Error("move response is invalid") };
    }
    return {
      status: "success",
      location: body.location,
      routes: body.routes,
      ap: body.ap,
    };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error("move request failed"),
    };
  }
}

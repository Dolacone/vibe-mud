export type CurrentUser = {
  id: string;
  display_name: string;
  email: string;
};

export type AuthResult =
  | { status: "authenticated"; user: CurrentUser }
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

function isCurrentUser(value: unknown): value is CurrentUser {
  if (typeof value !== "object" || value === null) return false;
  const user = value as Record<string, unknown>;
  return (
    typeof user.id === "string" &&
    typeof user.display_name === "string" &&
    typeof user.email === "string"
  );
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
      },
    };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error("identity request failed"),
    };
  }
}

import { describe, expect, it, vi } from "vitest";
import { getCurrentUser, rest } from "./auth";

describe("getCurrentUser", () => {
  it("asks the backend for the current identity with a same-origin credentialed request", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: 1, display_name: "Ada", email: "ada@example.com", ap: 3000, role: "admin" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(getCurrentUser(fetcher)).resolves.toEqual({
      status: "authenticated",
      user: { id: 1, display_name: "Ada", email: "ada@example.com", ap: 3000 },
    });
    expect(fetcher).toHaveBeenCalledWith("/api/me", {
      method: "GET",
      credentials: "include",
      headers: { Accept: "application/json" },
    });
  });

  it("reports a missing session without treating it as a backend error", async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(null, { status: 401 }));
    await expect(getCurrentUser(fetcher)).resolves.toEqual({ status: "unauthenticated" });
  });

  it("rejects other HTTP failures and malformed identities", async () => {
    const failed = vi.fn().mockResolvedValue(new Response(null, { status: 503 }));
    await expect(getCurrentUser(failed)).resolves.toMatchObject({ status: "error" });

    const malformed = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: "u-1", email: "ada@example.com", ap: 3000 }), { status: 200 }),
    );
    await expect(getCurrentUser(malformed)).resolves.toMatchObject({ status: "error" });
  });

  it("accepts numeric backend IDs and rejects string IDs", async () => {
    const numeric = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: 42, display_name: "Ada", email: "ada@example.com", ap: 0 }), { status: 200 }),
    );
    await expect(getCurrentUser(numeric)).resolves.toEqual({
      status: "authenticated",
      user: { id: 42, display_name: "Ada", email: "ada@example.com", ap: 0 },
    });

    const stringID = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: "42", display_name: "Ada", email: "ada@example.com", ap: 0 }), { status: 200 }),
    );
    await expect(getCurrentUser(stringID)).resolves.toMatchObject({ status: "error" });
  });

  it("does not read browser storage", async () => {
    const storageRead = vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("browser storage must not be read");
    });
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: 1, display_name: "Ada", email: "ada@example.com", ap: 1 }), { status: 200 }),
    );

    await expect(getCurrentUser(fetcher)).resolves.toMatchObject({ status: "authenticated" });
    expect(storageRead).not.toHaveBeenCalled();
    storageRead.mockRestore();
  });

  it.each([[-1, "negative"], [3001, "above maximum"], [1.5, "fractional"]])(
    "rejects %s AP in the identity response (%s)",
    async (ap) => {
      const fetcher = vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ id: 1, display_name: "Ada", email: "ada@example.com", ap }), { status: 200 }),
      );

      await expect(getCurrentUser(fetcher)).resolves.toMatchObject({ status: "error" });
    },
  );
});

describe("rest", () => {
  it("sends a same-origin credentialed POST and returns the decremented AP", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ ap: 2999 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(rest(fetcher)).resolves.toEqual({ status: "success", ap: 2999 });
    expect(fetcher).toHaveBeenCalledWith("/api/actions/rest", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json" },
    });
  });

  it("preserves the conflict error and AP for the UI", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "insufficient action points", ap: 0 }), { status: 409 }),
    );

    await expect(rest(fetcher)).resolves.toEqual({
      status: "insufficient",
      error: "insufficient action points",
      ap: 0,
    });
  });

  it("distinguishes an expired session from other HTTP failures", async () => {
    const unauthenticated = vi.fn().mockResolvedValue(new Response(null, { status: 401 }));
    await expect(rest(unauthenticated)).resolves.toEqual({ status: "unauthenticated" });

    const unavailable = vi.fn().mockResolvedValue(new Response(null, { status: 503 }));
    await expect(rest(unavailable)).resolves.toMatchObject({
      status: "error",
      error: new Error("rest request failed with status 503"),
    });
  });

  it("rejects invalid AP values in success and conflict contracts", async () => {
    for (const ap of [-1, 3001, 1.5, "2999"]) {
      const success = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ap }), { status: 200 }));
      await expect(rest(success)).resolves.toMatchObject({ status: "error" });

      const conflict = vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: "insufficient action points", ap }), { status: 409 }),
      );
      await expect(rest(conflict)).resolves.toMatchObject({ status: "error" });
    }
  });

  it("rejects a malformed conflict contract", async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ap: 0 }), { status: 409 }));
    await expect(rest(fetcher)).resolves.toMatchObject({ status: "error" });
  });

  it("requires the success contract to use HTTP 200", async () => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ap: 2999 }), { status: 201 }));
    await expect(rest(fetcher)).resolves.toMatchObject({ status: "error" });
  });
});

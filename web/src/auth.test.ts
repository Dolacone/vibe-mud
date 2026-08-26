import { describe, expect, it, vi } from "vitest";
import { convert, gather, getCurrentUser, move, rest, type PlayerState } from "./auth";

const campState: PlayerState = {
  location: { id: "camp", display_name: "Camp" },
  routes: [{ origin_id: "camp", destination_id: "forest_edge", ap_cost: 20 }],
  ap: 3000,
  inventory: [],
  gathering_option: null,
  conversion_option: {
    item: { id: "wood", display_name: "Wood" },
    input_quantity: 1,
    resource_yield: 1,
    ap_cost: 1,
  },
  resource: 0,
};

const forestState: PlayerState = {
  location: { id: "forest_edge", display_name: "Forest edge" },
  routes: [{ origin_id: "forest_edge", destination_id: "camp", ap_cost: 20 }],
  ap: 2980,
  inventory: [],
  gathering_option: {
    item: { id: "wood", display_name: "Wood" },
    quantity: 1,
    ap_cost: 10,
  },
  conversion_option: null,
  resource: 0,
};

const gatheredForestState: PlayerState = {
  ...forestState,
  ap: 2970,
  inventory: [{ item: { id: "wood", display_name: "Wood" }, quantity: 1 }],
};

describe("getCurrentUser", () => {
  it("asks the backend for the current identity with a same-origin credentialed request", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ id: 1, display_name: "Ada", email: "ada@example.com", ...campState, role: "admin" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(getCurrentUser(fetcher)).resolves.toEqual({
      status: "authenticated",
      user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState },
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
      new Response(JSON.stringify({ id: 42, display_name: "Ada", email: "ada@example.com", ...campState, ap: 0 }), { status: 200 }),
    );
    await expect(getCurrentUser(numeric)).resolves.toEqual({
      status: "authenticated",
      user: { id: 42, display_name: "Ada", email: "ada@example.com", ...campState, ap: 0 },
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
      new Response(JSON.stringify({ id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 1 }), { status: 200 }),
    );

    await expect(getCurrentUser(fetcher)).resolves.toMatchObject({ status: "authenticated" });
    expect(storageRead).not.toHaveBeenCalled();
    storageRead.mockRestore();
  });

  it.each([[-1, "negative"], [3001, "above maximum"], [1.5, "fractional"]])(
    "rejects %s AP in the identity response (%s)",
    async (ap) => {
      const fetcher = vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap }), { status: 200 }),
      );

      await expect(getCurrentUser(fetcher)).resolves.toMatchObject({ status: "error" });
    },
  );
});

describe("move", () => {
  it("sends only the target identifier and returns the authoritative state", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(forestState), { status: 200, headers: { "Content-Type": "application/json" } }),
    );

    await expect(move("forest_edge", fetcher)).resolves.toEqual({ status: "success", ...forestState });
    expect(fetcher).toHaveBeenCalledWith("/api/actions/move", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify({ target: "forest_edge" }),
    });
  });

  it("returns insufficient AP with the unchanged backend state", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "insufficient action points", ...campState, ap: 0 }), { status: 409 }),
    );

    await expect(move("forest_edge", fetcher)).resolves.toEqual({
      status: "insufficient",
      error: "insufficient action points",
      ...campState,
      ap: 0,
    });
  });

  it("returns invalid target with the backend state when provided", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "invalid target", ...campState }), { status: 400 }),
    );

    await expect(move("unknown", fetcher)).resolves.toEqual({
      status: "invalid",
      error: "invalid target",
      state: campState,
    });
  });

  it("returns invalid input without inventing missing state", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "invalid action input" }), { status: 400 }),
    );

    await expect(move("forest_edge", fetcher)).resolves.toEqual({ status: "invalid", error: "invalid action input" });
  });

  it("distinguishes an expired session from malformed and unavailable responses", async () => {
    const unauthenticated = vi.fn().mockResolvedValue(new Response(null, { status: 401 }));
    await expect(move("forest_edge", unauthenticated)).resolves.toEqual({ status: "unauthenticated" });

    const malformed = vi.fn().mockResolvedValue(new Response(JSON.stringify({ location: campState.location }), { status: 200 }));
    await expect(move("forest_edge", malformed)).resolves.toMatchObject({ status: "error" });

    const unavailable = vi.fn().mockResolvedValue(new Response(null, { status: 503 }));
    await expect(move("forest_edge", unavailable)).resolves.toMatchObject({
      status: "error",
      error: new Error("move request failed with status 503"),
    });
  });

  it("rejects a blank target before sending a request", async () => {
    const fetcher = vi.fn();
    await expect(move("  ", fetcher)).resolves.toEqual({ status: "invalid", error: "invalid target" });
    expect(fetcher).not.toHaveBeenCalled();
  });
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

describe("gather", () => {
  it("sends the only supported empty payload and returns authoritative inventory state", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(gatheredForestState), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(gather(fetcher)).resolves.toEqual({ status: "success", ...gatheredForestState });
    expect(fetcher).toHaveBeenCalledWith("/api/actions/gather", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: "{}",
    });
  });

  it("returns insufficient AP with the unchanged authoritative state", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "insufficient action points", ...forestState, ap: 0 }), { status: 409 }),
    );

    await expect(gather(fetcher)).resolves.toEqual({
      status: "insufficient",
      error: "insufficient action points",
      ...forestState,
      ap: 0,
    });
  });

  it.each([
    ["invalid location", "gathering not found", campState],
    ["invalid input", "invalid action input", forestState],
  ])("returns %s with the backend state", async (_label, error, state) => {
    const fetcher = vi.fn().mockResolvedValue(new Response(JSON.stringify({ error, ...state }), { status: 400 }));

    await expect(gather(fetcher)).resolves.toEqual({ status: "invalid", error, state });
  });

  it("distinguishes an expired session from malformed and unavailable responses", async () => {
    const unauthenticated = vi.fn().mockResolvedValue(new Response(null, { status: 401 }));
    await expect(gather(unauthenticated)).resolves.toEqual({ status: "unauthenticated" });

    const malformed = vi.fn().mockResolvedValue(new Response(JSON.stringify({ ap: gatheredForestState.ap }), { status: 200 }));
    await expect(gather(malformed)).resolves.toMatchObject({ status: "error" });

    const unavailable = vi.fn().mockResolvedValue(new Response(null, { status: 503 }));
    await expect(gather(unavailable)).resolves.toMatchObject({
      status: "error",
      error: new Error("gather request failed with status 503"),
    });
  });

  it("rejects malformed state contracts for inventory and gathering options", async () => {
    const malformedInventory = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ ...forestState, inventory: [{ item: forestState.gathering_option?.item, quantity: 0 }] }), { status: 200 }),
    );
    await expect(gather(malformedInventory)).resolves.toMatchObject({ status: "error" });

    const malformedOption = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ ...forestState, gathering_option: { ...forestState.gathering_option, ap_cost: 0 } }), { status: 200 }),
    );
    await expect(gather(malformedOption)).resolves.toMatchObject({ status: "error" });
  });
});

describe("convert", () => {
  const convertedCampState: PlayerState = {
    ...campState,
    ap: 2999,
    inventory: [],
    resource: 1,
  };

  it("sends only the supported empty payload and returns authoritative state", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(convertedCampState), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(convert(fetcher)).resolves.toEqual({ status: "success", ...convertedCampState });
    expect(fetcher).toHaveBeenCalledWith("/api/actions/convert", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: "{}",
    });
  });

  it.each([
    ["insufficient action points", { ...campState, ap: 0 }],
    ["insufficient item", campState],
  ])("returns an insufficient result with authoritative state for %s", async (error, state) => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error, ...state }), { status: 409 }),
    );

    await expect(convert(fetcher)).resolves.toEqual({ status: "insufficient", error, ...state });
  });

  it("returns invalid location with the unchanged backend state", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "conversion not found", ...forestState }), { status: 400 }),
    );

    await expect(convert(fetcher)).resolves.toEqual({
      status: "invalid",
      error: "conversion not found",
      state: forestState,
    });
  });

  it("returns invalid input without inventing missing state", async () => {
    const fetcher = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "invalid action input" }), { status: 400 }),
    );

    await expect(convert(fetcher)).resolves.toEqual({ status: "invalid", error: "invalid action input" });
  });

  it("distinguishes an expired session from malformed and unavailable responses", async () => {
    const unauthenticated = vi.fn().mockResolvedValue(new Response(null, { status: 401 }));
    await expect(convert(unauthenticated)).resolves.toEqual({ status: "unauthenticated" });

    const malformed = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ ...convertedCampState, resource: -1 }), { status: 200 }),
    );
    await expect(convert(malformed)).resolves.toMatchObject({
      status: "error",
      error: new Error("convert response is invalid"),
    });

    const unavailable = vi.fn().mockResolvedValue(new Response(null, { status: 503 }));
    await expect(convert(unavailable)).resolves.toMatchObject({
      status: "error",
      error: new Error("convert request failed with status 503"),
    });
  });

  it("rejects malformed conflict and invalid-location response contracts", async () => {
    const malformedConflict = vi.fn().mockResolvedValue(
      new Response(JSON.stringify({ error: "insufficient item", resource: 1 }), { status: 409 }),
    );
    await expect(convert(malformedConflict)).resolves.toMatchObject({ status: "error" });

    const malformedInvalid = vi.fn().mockResolvedValue(new Response(JSON.stringify({ resource: 1 }), { status: 400 }));
    await expect(convert(malformedInvalid)).resolves.toMatchObject({ status: "error" });
  });
});

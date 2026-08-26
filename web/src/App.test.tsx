import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import * as auth from "./auth";

vi.mock("./auth", async () => {
  const actual = await vi.importActual<typeof import("./auth")>("./auth");
  return { ...actual, getCurrentUser: vi.fn(), rest: vi.fn(), move: vi.fn(), gather: vi.fn(), convert: vi.fn() };
});

const getCurrentUser = vi.mocked(auth.getCurrentUser);
const rest = vi.mocked(auth.rest);
const move = vi.mocked(auth.move);
const gather = vi.mocked(auth.gather);
const convert = vi.mocked(auth.convert);

const campState = {
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

const forestState = {
  location: { id: "forest_edge", display_name: "Forest edge" },
  routes: [{ origin_id: "forest_edge", destination_id: "camp", ap_cost: 20 }],
  ap: 2980,
  inventory: [],
  gathering_option: { item: { id: "wood", display_name: "Wood" }, quantity: 1, ap_cost: 10 },
  conversion_option: null,
  resource: 0,
};

describe("App", () => {
  beforeEach(() => {
    getCurrentUser.mockReset();
    rest.mockReset();
    move.mockReset();
    gather.mockReset();
    convert.mockReset();
  });

  it("loads and displays only the backend-confirmed identity", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    render(<App />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading");
    await waitFor(() => expect(screen.getByText("Ada")).toBeInTheDocument());
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("ada@example.com")).toBeInTheDocument();
    expect(screen.getByText("AP: 3000")).toBeInTheDocument();
    expect(screen.getByText("Resource: 0")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled();
    expect(screen.getByText("Current location: Camp")).toBeInTheDocument();
    expect(screen.getByText("To forest_edge (20 AP)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Move to forest_edge" })).toBeEnabled();
    expect(screen.getByText("Inventory is empty.")).toBeInTheDocument();
    expect(screen.getByText("No gathering action available.")).toBeInTheDocument();
    expect(screen.getByText("Input: 1 Wood; Yield: 1 Resource; Cost: 1 AP")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Convert" })).toBeEnabled();
    expect(screen.queryByText(/role|token/i)).not.toBeInTheDocument();
  });

  it("shows the backend gathering option and applies the authoritative state after success", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...forestState } });
    gather.mockResolvedValue({
      status: "success",
      ...forestState,
      ap: 2970,
      inventory: [{ item: { id: "wood", display_name: "Oak wood" }, quantity: 7 }],
      gathering_option: { item: { id: "wood", display_name: "Oak wood" }, quantity: 3, ap_cost: 12 },
    });
    render(<App />);

    expect(await screen.findByRole("button", { name: "Gather" })).toBeEnabled();
    expect(screen.getByText("Yield: 1 Wood; Cost: 10 AP")).toBeInTheDocument();
    (await screen.findByRole("button", { name: "Gather" })).click();
    await waitFor(() => expect(screen.getByText("Gather succeeded.")).toBeInTheDocument());
    expect(screen.getByText("AP: 2970")).toBeInTheDocument();
    expect(screen.getByText("Oak wood: 7")).toBeInTheDocument();
    expect(screen.getByText("Yield: 3 Oak wood; Cost: 12 AP")).toBeInTheDocument();
    expect(gather).toHaveBeenCalledTimes(1);
  });

  it("applies the authoritative state after an insufficient gather", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...forestState, ap: 1 } });
    gather.mockResolvedValue({
      status: "insufficient",
      error: "insufficient action points",
      ...forestState,
      ap: 0,
      inventory: [{ item: { id: "wood", display_name: "Wood" }, quantity: 4 }],
    });
    render(<App />);

    (await screen.findByRole("button", { name: "Gather" })).click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByText("AP: 0")).toBeInTheDocument();
    expect(screen.getByText("Wood: 4")).toBeInTheDocument();
  });

  it("disables every action and prevents duplicate gathers while one is pending", async () => {
    let resolveGather: ((value: auth.GatherResult) => void) | undefined;
    gather.mockReturnValue(new Promise((resolve) => { resolveGather = resolve; }));
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...forestState } });
    render(<App />);

    const gatherButton = await screen.findByRole("button", { name: "Gather" });
    gatherButton.click();
    await waitFor(() => expect(gatherButton).toBeDisabled());
    expect(screen.getAllByRole("button").every((button) => button.hasAttribute("disabled"))).toBe(true);
    gatherButton.click();
    expect(gather).toHaveBeenCalledTimes(1);

    resolveGather?.({ status: "success", ...forestState, ap: 2970, inventory: [{ item: { id: "wood", display_name: "Wood" }, quantity: 1 }] });
    await waitFor(() => expect(screen.getByText("Wood: 1")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Gather" })).toBeEnabled();
  });

  it("applies the authoritative state after converting the last Wood", async () => {
    const stateWithWood = {
      ...campState,
      inventory: [{ item: { id: "wood", display_name: "Wood" }, quantity: 1 }],
    };
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...stateWithWood } });
    convert.mockResolvedValue({ status: "success", ...campState, ap: 2999, resource: 1 });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Convert" });
    button.click();
    await waitFor(() => expect(screen.getByText("Convert succeeded.")).toBeInTheDocument());
    expect(screen.getByText("AP: 2999")).toBeInTheDocument();
    expect(screen.getByText("Resource: 1")).toBeInTheDocument();
    expect(screen.getByText("Inventory is empty.")).toBeInTheDocument();
    expect(screen.queryByText("Wood: 1")).not.toBeInTheDocument();
    expect(convert).toHaveBeenCalledTimes(1);
  });

  it("applies the authoritative state after an unsuccessful conversion", async () => {
    const stateWithWood = {
      ...campState,
      ap: 0,
      inventory: [{ item: { id: "wood", display_name: "Wood" }, quantity: 2 }],
      resource: 3,
    };
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...stateWithWood } });
    convert.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...stateWithWood });
    render(<App />);

    (await screen.findByRole("button", { name: "Convert" })).click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByText("AP: 0")).toBeInTheDocument();
    expect(screen.getByText("Resource: 3")).toBeInTheDocument();
    expect(screen.getByText("Wood: 2")).toBeInTheDocument();
  });

  it("disables every action and prevents duplicate conversions while one is pending", async () => {
    let resolveConvert: ((value: auth.ConvertResult) => void) | undefined;
    convert.mockReturnValue(new Promise((resolve) => { resolveConvert = resolve; }));
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Convert" });
    button.click();
    await waitFor(() => expect(button).toBeDisabled());
    expect(screen.getAllByRole("button").every((action) => action.hasAttribute("disabled"))).toBe(true);
    button.click();
    expect(convert).toHaveBeenCalledTimes(1);

    resolveConvert?.({ status: "success", ...campState, ap: 2999, resource: 1 });
    await waitFor(() => expect(screen.getByText("Resource: 1")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Convert" })).toBeEnabled();
  });

  it("applies the authoritative state after a successful move", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    move.mockResolvedValue({
      status: "success",
      location: { id: "forest_edge", display_name: "Forest edge" },
      routes: [{ origin_id: "forest_edge", destination_id: "camp", ap_cost: 20 }],
      ap: 2980,
      inventory: [],
      gathering_option: null,
      conversion_option: null,
      resource: 0,
    });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Move to forest_edge" });
    button.click();
    await waitFor(() => expect(screen.getByText("Move succeeded. Current location: Forest edge")).toBeInTheDocument());
    expect(screen.getByText("Current location: Forest edge")).toBeInTheDocument();
    expect(screen.getByText("To camp (20 AP)")).toBeInTheDocument();
    expect(screen.getByText("AP: 2980")).toBeInTheDocument();
    expect(move).toHaveBeenCalledWith("forest_edge");
  });

  it("keeps the authoritative state after an insufficient move", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 10 } });
    move.mockResolvedValue({
      status: "insufficient",
      error: "insufficient action points",
      ...campState,
      ap: 10,
    });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Move to forest_edge" });
    button.click();
    await waitFor(() => expect(screen.getByText("Move failed: insufficient action points")).toBeInTheDocument());
    expect(screen.getByText("Current location: Camp")).toBeInTheDocument();
    expect(screen.getByText("AP: 10")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Move to forest_edge" })).toBeEnabled();
  });

  it("prevents duplicate move requests while one is pending", async () => {
    let resolveMove: ((value: auth.MoveResult) => void) | undefined;
    move.mockReturnValue(new Promise((resolve) => { resolveMove = resolve; }));
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Move to forest_edge" });
    button.click();
    await waitFor(() => expect(button).toBeDisabled());
    expect(move).toHaveBeenCalledTimes(1);
    button.click();
    expect(move).toHaveBeenCalledTimes(1);

    resolveMove?.({
      status: "success",
      location: { id: "forest_edge", display_name: "Forest edge" },
      routes: [{ origin_id: "forest_edge", destination_id: "camp", ap_cost: 20 }],
      ap: 2980,
      inventory: [],
      gathering_option: null,
      conversion_option: null,
      resource: 0,
    });
    await waitFor(() => expect(screen.getByText("Current location: Forest edge")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Move to camp" })).toBeEnabled();
  });

  it("updates the displayed AP after a successful rest", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 2 } });
    rest.mockResolvedValue({ status: "success", ap: 1 });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(screen.getByText("Rest succeeded. AP: 1")).toBeInTheDocument());
    expect(screen.getByText("AP: 1")).toBeInTheDocument();
    expect(rest).toHaveBeenCalledTimes(1);
  });

  it("keeps the known AP and shows the rejection when AP is insufficient", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 0 } });
    rest.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ap: 0 });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByText("AP: 0")).toBeInTheDocument();
  });

  it("updates stale AP from an insufficient rest response", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 1 } });
    rest.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ap: 0 });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    expect(screen.getByText("AP: 1")).toBeInTheDocument();
    button.click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByText("AP: 0")).toBeInTheDocument();
    expect(screen.queryByText("AP: 1")).not.toBeInTheDocument();
  });

  it("disables rest and prevents duplicate requests while one is pending", async () => {
    let resolveRest: ((value: auth.RestResult) => void) | undefined;
    rest.mockReturnValue(new Promise((resolve) => { resolveRest = resolve; }));
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 2 } });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(button).toBeDisabled());
    expect(rest).toHaveBeenCalledTimes(1);
    button.click();
    expect(rest).toHaveBeenCalledTimes(1);

    resolveRest?.({ status: "success", ap: 1 });
    await waitFor(() => expect(screen.getByText("AP: 1")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled();
  });

  it("keeps the known AP when rest fails", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 2 } });
    rest.mockResolvedValue({ status: "error", error: new Error("backend unavailable") });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("backend unavailable"));
    expect(screen.getByText("AP: 2")).toBeInTheDocument();
  });

  it("offers same-origin Google login when signed out", async () => {
    getCurrentUser.mockResolvedValue({ status: "unauthenticated" });
    render(<App />);
    await waitFor(() => expect(screen.getByRole("heading", { name: "Not signed in" })).toBeInTheDocument());
    expect(screen.getByRole("link", { name: /google/i })).toHaveAttribute("href", "/auth/google/login");
  });

  it("does not retain identity after an unauthenticated response", async () => {
    getCurrentUser.mockResolvedValue({ status: "unauthenticated" });
    render(<App />);
    await waitFor(() => expect(screen.queryByText("Ada")).not.toBeInTheDocument());
    expect(screen.getByRole("heading", { name: "Not signed in" })).toBeInTheDocument();
  });

  it("shows backend failures separately", async () => {
    getCurrentUser.mockResolvedValue({ status: "error", error: new Error("backend unavailable") });
    render(<App />);
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("backend unavailable"));
    expect(screen.getByRole("link", { name: /google/i })).toBeInTheDocument();
  });
});

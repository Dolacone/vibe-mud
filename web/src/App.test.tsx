import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import * as auth from "./auth";
import "./styles.css";

vi.mock("./auth", async () => {
  const actual = await vi.importActual<typeof import("./auth")>("./auth");
  return { ...actual, getCurrentUser: vi.fn(), rest: vi.fn(), move: vi.fn(), gather: vi.fn(), convert: vi.fn(), craft: vi.fn(), build: vi.fn(), contributeConstruction: vi.fn(), repairBuilding: vi.fn(), drop: vi.fn(), pickup: vi.fn() };
});

const getCurrentUser = vi.mocked(auth.getCurrentUser);
const rest = vi.mocked(auth.rest);
const move = vi.mocked(auth.move);
const gather = vi.mocked(auth.gather);
const convert = vi.mocked(auth.convert);
const craft = vi.mocked(auth.craft);
const build = vi.mocked(auth.build);
const contributeConstruction = vi.mocked(auth.contributeConstruction);
const repairBuilding = vi.mocked(auth.repairBuilding);
const drop = vi.mocked(auth.drop);
const pickup = vi.mocked(auth.pickup);

const resources = ["food", "wood", "stone", "metal", "fiber", "hide", "medicinal", "arcane"].map((id) => ({
  resource: { id, display_name: id[0].toUpperCase() + id.slice(1) },
  quantity: 0,
}));

const resourcesWithWood = (quantity: number) => resources.map((entry) => (
  entry.resource.id === "wood" ? { ...entry, quantity } : entry
));

const woodComponentRecipe = {
  id: "wood_component",
  display_name: "Wood Component",
  base_ap_cost: 10,
  resource_inputs: [{ resource: { id: "wood", display_name: "Wood" }, quantity: 10 }],
  item_inputs: [],
  output: { id: "wood_component", display_name: "Wood Component" },
  output_quantity: 1,
};

const buildingRecipe = {
  id: "building_lv1",
  display_name: "Building Lv1",
  building_level: 1,
  required_ap: 60,
  extension_slot_count: 1,
  resource_inputs: [{ resource: { id: "wood", display_name: "Wood" }, quantity: 10 }],
  item_inputs: [{ item: { id: "wood_component", display_name: "Wood Component" }, quantity: 1 }],
};

const underConstruction = {
  id: 1,
  owner: { id: 1, display_name: "Ada" },
  recipe: { id: "building_lv1", display_name: "Building Lv1" },
  building_level: 1,
  required_ap: 60,
  contributed_ap: 0,
  status: "under_construction" as const,
  extension_slot_count: 1,
  max_durability_seconds: 604800,
  durability_status: null,
  durability_remaining_seconds: null,
};

const completedActive = {
  ...underConstruction,
  contributed_ap: 60,
  status: "completed" as const,
  durability_status: "active" as const,
  durability_remaining_seconds: 604700,
};

const completedDisabled = {
  ...completedActive,
  durability_status: "disabled" as const,
  durability_remaining_seconds: 0,
};

const campState = {
  location: { id: "camp", display_name: "Camp" },
  routes: [{ origin_id: "camp", destination_id: "forest_edge", ap_cost: 20 }],
  ap: 3000,
  carried_weight: 0,
  movement_weight_threshold: 1000,
  inventory: [],
  ground_items: [],
  ground_resources: [],
  gathering_option: null,
  conversion_option: {
    item: { id: "wood", display_name: "Wood" },
    resource: { id: "wood", display_name: "Wood" },
    input_quantity: 1,
    resource_yield: 1,
    ap_cost: 1,
  },
  resources,
  crafting_recipes: [woodComponentRecipe],
  building_recipes: [buildingRecipe],
  buildings: [],
};

const forestState = {
  location: { id: "forest_edge", display_name: "Forest edge" },
  routes: [{ origin_id: "forest_edge", destination_id: "camp", ap_cost: 20 }],
  ap: 2980,
  carried_weight: 0,
  movement_weight_threshold: 1000,
  inventory: [],
  ground_items: [],
  ground_resources: [],
  gathering_option: { item: { id: "wood", display_name: "Wood" }, quantity: 1, ap_cost: 10 },
  conversion_option: null,
  resources,
  crafting_recipes: [woodComponentRecipe],
  building_recipes: [buildingRecipe],
  buildings: [],
};

const transferState = {
  ...campState,
  inventory: [{ item: { id: "wood", display_name: "Wood" }, quantity: 4 }],
  resources: resourcesWithWood(6),
  ground_items: [{ item: { id: "wood_component", display_name: "Wood Component" }, quantity: 2 }],
  ground_resources: [{ resource: { id: "stone", display_name: "Stone" }, quantity: 3 }],
};

const apRow = (value: number) => within(screen.getByRole("table", { name: "Player summary" })).getByRole("row", { name: new RegExp(`AP.*${value}`) });

describe("App", () => {
  beforeEach(() => {
    getCurrentUser.mockReset();
    rest.mockReset();
    move.mockReset();
    gather.mockReset();
    convert.mockReset();
    craft.mockReset();
    build.mockReset();
    contributeConstruction.mockReset();
    repairBuilding.mockReset();
    drop.mockReset();
    pickup.mockReset();
  });

  it("renders the authenticated game state as named compact tables", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    render(<App />);

    expect(await screen.findByRole("table", { name: "Player summary" })).toBeInTheDocument();
    for (const name of ["Actions", "Resources", "Available routes", "Gather", "Convert", "Craft", "Building recipes", "Buildings", "Ground Items", "Ground Resources", "Inventory"]) {
      expect(screen.getByRole("table", { name })).toBeInTheDocument();
    }
    expect(screen.getAllByRole("table")).toHaveLength(12);
    expect(screen.getAllByRole("table").every((table) => table.parentElement?.classList.contains("table-scroll"))).toBe(true);
    expect(screen.getByRole("table", { name: "Player summary" }).querySelectorAll('th[scope="row"]')).toHaveLength(7);
    const summary = screen.getByRole("table", { name: "Player summary" });
    expect(within(summary).getByRole("row", { name: /Carrying weight\s*0/ })).toBeInTheDocument();
    expect(within(summary).getByRole("row", { name: /Movement weight threshold\s*1000/ })).toBeInTheDocument();
    for (const name of ["Actions", "Resources", "Available routes", "Gather", "Convert", "Craft", "Building recipes", "Buildings", "Ground Items", "Ground Resources", "Inventory"]) {
      const headers = screen.getByRole("table", { name }).querySelectorAll("thead th");
      expect(headers.length).toBeGreaterThan(0);
      expect([...headers].every((header) => header.getAttribute("scope") === "col")).toBe(true);
    }
    expect(screen.getByRole("table", { name: "Resources" }).querySelectorAll('thead th[scope="col"]')).toHaveLength(3);
    expect(screen.getByRole("table", { name: "Craft" }).querySelectorAll("tbody > tr")).toHaveLength(1);
    expect(screen.getByRole("table", { name: "Craft" })).toHaveTextContent("Wood: 10");
    expect(screen.getByRole("table", { name: "Craft" })).toHaveTextContent("Wood Component: 1");
    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Inventory is empty.");
    expect(screen.getByRole("table", { name: "Buildings" })).toHaveTextContent("No buildings at this location.");
  });

  it("keeps empty gameplay sections as explicit table rows", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, routes: [], gathering_option: null, conversion_option: null, crafting_recipes: [], building_recipes: [], buildings: [] } });
    render(<App />);

    expect(await screen.findByRole("table", { name: "Available routes" })).toHaveTextContent("No available routes.");
    expect(screen.getByRole("table", { name: "Gather" })).toHaveTextContent("No gathering action available.");
    expect(screen.getByRole("table", { name: "Convert" })).toHaveTextContent("No conversion action available.");
    expect(screen.getByRole("table", { name: "Craft" })).toHaveTextContent("No crafting recipes available.");
    expect(screen.getByRole("table", { name: "Building recipes" })).toHaveTextContent("No building recipes available.");
    expect(screen.getByRole("table", { name: "Buildings" })).toHaveTextContent("No buildings at this location.");
    expect(screen.getByRole("table", { name: "Ground Items" })).toHaveTextContent("Ground items are empty.");
    expect(screen.getByRole("table", { name: "Ground Resources" })).toHaveTextContent("Ground resources are empty.");
    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Inventory is empty.");
  });

  it("shows carrying weight and threshold so players can judge movement availability", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, carried_weight: 1000 } });
    render(<App />);

    const summary = await screen.findByRole("table", { name: "Player summary" });
    expect(within(summary).getByRole("row", { name: /Carrying weight\s*1000/ })).toBeInTheDocument();
    expect(within(summary).getByRole("row", { name: /Movement weight threshold\s*1000/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Move to forest_edge" })).toBeEnabled();
    expect(screen.queryByRole("alert", { name: "Cannot move while overweight." })).not.toBeInTheDocument();
  });

  it("blocks route controls while overweight so players cannot start a move the backend must reject", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, carried_weight: 1001 } });
    render(<App />);

    expect(await screen.findByRole("alert")).toHaveTextContent("Cannot move while overweight.");
    expect(screen.getByRole("button", { name: "Move to forest_edge" })).toBeDisabled();
    expect(move).not.toHaveBeenCalled();
  });

  it("renders public ground items and resources with pickup controls", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...transferState } });
    render(<App />);

    const groundItems = await screen.findByRole("table", { name: "Ground Items" });
    const groundResources = screen.getByRole("table", { name: "Ground Resources" });
    expect(groundItems).toHaveTextContent("Wood Component");
    expect(groundItems).toHaveTextContent("2");
    expect(within(groundItems).getByRole("spinbutton", { name: "Pickup quantity for Wood Component" })).toHaveValue(1);
    expect(within(groundItems).getByRole("button", { name: "Pickup Wood Component" })).toBeEnabled();
    expect(groundResources).toHaveTextContent("Stone");
    expect(groundResources).toHaveTextContent("3");
    expect(within(groundResources).getByRole("spinbutton", { name: "Pickup quantity for Stone" })).toHaveValue(1);
    expect(within(groundResources).getByRole("button", { name: "Pickup Stone" })).toBeEnabled();
  });

  it("drops an Item with the requested quantity and applies authoritative state", async () => {
    const nextState = { ...transferState, inventory: [{ item: { id: "wood", display_name: "Wood" }, quantity: 2 }], ground_items: [{ item: { id: "wood_component", display_name: "Wood Component" }, quantity: 4 }] };
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...transferState } });
    drop.mockResolvedValue({ status: "success", ...nextState });
    render(<App />);

    const inventory = await screen.findByRole("table", { name: "Inventory" });
    const input = within(inventory).getByRole("spinbutton", { name: "Drop quantity for Wood" });
    fireEvent.change(input, { target: { value: "2" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByText("Drop succeeded.")).toBeInTheDocument());
    expect(drop).toHaveBeenCalledWith({ asset_type: "item", asset_id: "wood", quantity: 2 });
    expect(within(inventory).getByText("Wood: 2")).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Ground Items" })).getByText("4")).toBeInTheDocument();
  });

  it("drops a Resource with a positive integer quantity", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...transferState } });
    drop.mockResolvedValue({ status: "success", ...transferState, resources: resourcesWithWood(4), ground_resources: [{ resource: { id: "stone", display_name: "Stone" }, quantity: 5 }] });
    render(<App />);

    const resourcesTable = await screen.findByRole("table", { name: "Resources" });
    const input = within(resourcesTable).getByRole("spinbutton", { name: "Drop quantity for Wood" });
    fireEvent.change(input, { target: { value: "2" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByText("Drop succeeded.")).toBeInTheDocument());
    expect(drop).toHaveBeenCalledWith({ asset_type: "resource", asset_id: "wood", quantity: 2 });
    expect(within(resourcesTable).getByText("Wood: 4")).toBeInTheDocument();
  });

  it("picks up an Item with the requested quantity", async () => {
    const nextState = { ...transferState, inventory: [{ item: { id: "wood", display_name: "Wood" }, quantity: 5 }], ground_items: [{ item: { id: "wood_component", display_name: "Wood Component" }, quantity: 1 }] };
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...transferState } });
    pickup.mockResolvedValue({ status: "success", ...nextState });
    render(<App />);

    const groundItems = await screen.findByRole("table", { name: "Ground Items" });
    const input = within(groundItems).getByRole("spinbutton", { name: "Pickup quantity for Wood Component" });
    fireEvent.change(input, { target: { value: "1" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByText("Pickup succeeded.")).toBeInTheDocument());
    expect(pickup).toHaveBeenCalledWith({ asset_type: "item", asset_id: "wood_component", quantity: 1 });
    expect(within(groundItems).getByText("1")).toBeInTheDocument();
  });

  it("applies authoritative state after an unsuccessful Resource pickup", async () => {
    const nextState = { ...transferState, ap: 2998, resources: resourcesWithWood(8), ground_resources: [{ resource: { id: "stone", display_name: "Stone" }, quantity: 1 }] };
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...transferState } });
    pickup.mockResolvedValue({ status: "conflict", error: "insufficient ground quantity", ...nextState });
    render(<App />);

    const groundResources = await screen.findByRole("table", { name: "Ground Resources" });
    fireEvent.submit(within(groundResources).getByRole("spinbutton", { name: "Pickup quantity for Stone" }).closest("form")!);
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient ground quantity"));
    expect(pickup).toHaveBeenCalledWith({ asset_type: "resource", asset_id: "stone", quantity: 1 });
    expect(apRow(2998)).toBeInTheDocument();
    expect(within(groundResources).getByText("1")).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Resources" })).toHaveTextContent("Wood: 8");
  });

  it("prevents duplicate transfers while one request is pending", async () => {
    let resolveDrop: ((value: auth.TransferResult) => void) | undefined;
    drop.mockReturnValue(new Promise((resolve) => { resolveDrop = resolve; }));
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...transferState } });
    render(<App />);

    const inventory = await screen.findByRole("table", { name: "Inventory" });
    const input = within(inventory).getByRole("spinbutton", { name: "Drop quantity for Wood" });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(within(inventory).getByRole("button", { name: "Drop Wood" })).toBeDisabled());
    fireEvent.submit(input.closest("form")!);
    expect(drop).toHaveBeenCalledTimes(1);
    resolveDrop?.({ status: "success", ...transferState });
    await waitFor(() => expect(screen.getByText("Drop succeeded.")).toBeInTheDocument());
  });

  it("keeps tables inside the page width and gives gameplay headers column scope", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    render(<App />);

    await screen.findByRole("table", { name: "Player summary" });
    const main = document.querySelector("main")!;
    const wrapper = document.querySelector(".table-scroll")!;
    const table = screen.getByRole("table", { name: "Craft" });
    expect(getComputedStyle(main).boxSizing).toBe("border-box");
    expect(getComputedStyle(main).width).toBe("100%");
    expect(getComputedStyle(wrapper).maxWidth).toBe("100%");
    expect(getComputedStyle(wrapper).overflowX).toBe("auto");
    expect(getComputedStyle(table).width).toBe("100%");
    expect(getComputedStyle(table).minWidth).toBe("max-content");
    expect(table.querySelectorAll('thead th[scope="col"]')).toHaveLength(6);
  });

  it("shows active Building durability and a repair action", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, buildings: [completedActive] } });
    render(<App />);

    expect(await screen.findByText("Durability status: active")).toBeInTheDocument();
    expect(screen.getByText("Remaining durability: 604700 seconds")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Repair building 1" })).toBeEnabled();
  });

  it("shows disabled Building durability with zero remaining seconds", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, buildings: [completedDisabled] } });
    render(<App />);

    expect(await screen.findByText("Durability status: disabled")).toBeInTheDocument();
    expect(screen.getByText("Remaining durability: 0 seconds")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Repair building 1" })).toBeEnabled();
  });

  it("repairs a completed Building and applies the authoritative state", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, buildings: [completedActive] } });
    repairBuilding.mockResolvedValue({ status: "success", ...campState, ap: 2990, buildings: [{ ...completedActive, durability_remaining_seconds: 604800 }] });
    render(<App />);

    (await screen.findByRole("button", { name: "Repair building 1" })).click();
    await waitFor(() => expect(screen.getByText("Building repair succeeded.")).toBeInTheDocument());
    expect(repairBuilding).toHaveBeenCalledWith(1);
    expect(apRow(2990)).toBeInTheDocument();
    expect(screen.getByText("Remaining durability: 604800 seconds")).toBeInTheDocument();
  });

  it("shows a repair failure and applies its authoritative state", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, buildings: [completedActive] } });
    repairBuilding.mockResolvedValue({ status: "conflict", error: "insufficient action points", ...campState, ap: 5, buildings: [completedDisabled] });
    render(<App />);

    (await screen.findByRole("button", { name: "Repair building 1" })).click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(apRow(5)).toBeInTheDocument();
    expect(screen.getByText("Durability status: disabled")).toBeInTheDocument();
    expect(screen.getByText("Remaining durability: 0 seconds")).toBeInTheDocument();
  });

  it("displays the backend building recipe and current-location construction state", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, buildings: [underConstruction] } });
    render(<App />);

    expect(await screen.findByRole("table", { name: "Building recipes" })).toHaveTextContent("Building Lv1");
    expect(screen.getByRole("table", { name: "Building recipes" })).toHaveTextContent("60");
    expect(screen.getByText("Wood Component: 1")).toBeInTheDocument();
    expect(screen.getByText("Owner: Ada")).toBeInTheDocument();
    expect(screen.getByText("Status: under_construction")).toBeInTheDocument();
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
    expect(screen.getByText("Empty extension slots: 1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Build Building Lv1" })).toBeEnabled();
  });

  it("starts construction with only the backend recipe identifier and applies authoritative state", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    build.mockResolvedValue({ status: "success", ...campState, ap: 3000, inventory: [], buildings: [underConstruction] });
    render(<App />);

    (await screen.findByRole("button", { name: "Build Building Lv1" })).click();
    await waitFor(() => expect(screen.getByText("Building construction started.")).toBeInTheDocument());
    expect(build).toHaveBeenCalledWith("building_lv1");
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
  });

  it("restores the persisted building after a page reload", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, buildings: [underConstruction] } });
    const view = render(<App />);
    expect(await screen.findByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
    view.unmount();
    render(<App />);
    expect(await screen.findByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
  });

  it("shows an occupied-slot failure with authoritative state", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, buildings: [underConstruction] } });
    build.mockResolvedValue({ status: "invalid", error: "building already exists", ...campState, buildings: [underConstruction] });
    render(<App />);

    (await screen.findByRole("button", { name: "Build Building Lv1" })).click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("building already exists"));
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
  });

  it("allows same-location contribution and caps oversized AP at completion", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 100, buildings: [underConstruction] } });
    contributeConstruction.mockResolvedValue({ status: "success", ...campState, ap: 40, buildings: [{ ...underConstruction, contributed_ap: 60, status: "completed" as const }] });
    render(<App />);

    const input = await screen.findByRole("spinbutton", { name: "Contribution AP for building 1" });
    fireEvent.change(input, { target: { value: "100" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByText("Construction contribution succeeded.")).toBeInTheDocument());
    expect(contributeConstruction).toHaveBeenCalledWith(1, 100);
    expect(screen.getByText("Status: completed")).toBeInTheDocument();
    expect(screen.getByText("Progress: 60/60 AP (100%)")).toBeInTheDocument();
    expect(screen.queryByRole("spinbutton", { name: "Contribution AP for building 1" })).not.toBeInTheDocument();
  });

  it("prevents duplicate building actions while a request is pending", async () => {
    let resolveBuild: ((value: auth.BuildResult) => void) | undefined;
    build.mockReturnValue(new Promise((resolve) => { resolveBuild = resolve; }));
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Build Building Lv1" });
    button.click();
    await waitFor(() => expect(button).toBeDisabled());
    button.click();
    expect(build).toHaveBeenCalledTimes(1);
    resolveBuild?.({ status: "success", ...campState, buildings: [underConstruction] });
    await waitFor(() => expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument());
  });

  it("keeps authoritative AP and progress after a failed contribution", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 5, buildings: [underConstruction] } });
    contributeConstruction.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...campState, ap: 5, buildings: [underConstruction] });
    render(<App />);

    const input = await screen.findByRole("spinbutton", { name: "Contribution AP for building 1" });
    fireEvent.change(input, { target: { value: "10" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(apRow(5)).toBeInTheDocument();
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
  });

  it("loads and displays only the backend-confirmed identity", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    render(<App />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading");
    await waitFor(() => expect(screen.getByText("Ada")).toBeInTheDocument());
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("ada@example.com")).toBeInTheDocument();
    expect(apRow(3000)).toBeInTheDocument();
    const resourcesTable = screen.getByRole("table", { name: "Resources" });
    expect(resourcesTable.querySelectorAll("tbody > tr")).toHaveLength(8);
    expect(resourcesTable).toHaveTextContent("Wood: 0");
    expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled();
    expect(screen.getByText("Current location: Camp")).toBeInTheDocument();
    expect(screen.getByText("To forest_edge (20 AP)")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Move to forest_edge" })).toBeEnabled();
    expect(screen.getByText("Inventory is empty.")).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Craft" })).toHaveTextContent("Wood Component");
    expect(screen.getByText("AP cost: 10")).toBeInTheDocument();
    expect(screen.getAllByText("Wood: 10")).toHaveLength(2);
    expect(screen.getByText("Output: Wood Component: 1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Craft Wood Component" })).toBeEnabled();
    expect(screen.getByText("No gathering action available.")).toBeInTheDocument();
    expect(screen.getByText("Input: 1 Wood; Yield: 1 Wood Resource; Cost: 1 AP")).toBeInTheDocument();
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
    expect(apRow(2970)).toBeInTheDocument();
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
    expect(apRow(0)).toBeInTheDocument();
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
    convert.mockResolvedValue({ status: "success", ...campState, ap: 2999, resources: resourcesWithWood(1) });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Convert" });
    button.click();
    await waitFor(() => expect(screen.getByText("Convert succeeded.")).toBeInTheDocument());
    expect(apRow(2999)).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Resources" })).toHaveTextContent("Wood: 1");
    expect(screen.getByText("Inventory is empty.")).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Inventory is empty.");
    expect(convert).toHaveBeenCalledTimes(1);
  });

  it("crafts by recipe identifier and displays authoritative state", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, resources: resourcesWithWood(10) } });
    craft.mockResolvedValue({
      status: "success",
      ...campState,
      ap: 2990,
      resources: resourcesWithWood(0),
      inventory: [{ item: woodComponentRecipe.output, quantity: 1 }],
    });
    render(<App />);

    (await screen.findByRole("button", { name: "Craft Wood Component" })).click();
    await waitFor(() => expect(screen.getByText("Craft succeeded.")).toBeInTheDocument());
    expect(apRow(2990)).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Resources" })).toHaveTextContent("Wood: 0");
    expect(screen.getAllByText("Wood Component: 1").length).toBeGreaterThan(0);
    expect(craft).toHaveBeenCalledWith("wood_component");
  });

  it("displays the recipe at a non-camp location", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...forestState } });
    render(<App />);

    expect(await screen.findByRole("table", { name: "Craft" })).toHaveTextContent("Wood Component");
    expect(screen.getByText("Current location: Forest edge")).toBeInTheDocument();
  });

  it("keeps authoritative state after a failed craft", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 5, resources: resourcesWithWood(2) } });
    craft.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...campState, ap: 5, resources: resourcesWithWood(2) });
    render(<App />);

    (await screen.findByRole("button", { name: "Craft Wood Component" })).click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(apRow(5)).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Resources" })).toHaveTextContent("Wood: 2");
    expect(screen.getByText("Inventory is empty.")).toBeInTheDocument();
  });

  it("prevents duplicate crafts while one is pending", async () => {
    let resolveCraft: ((value: auth.CraftResult) => void) | undefined;
    craft.mockReturnValue(new Promise((resolve) => { resolveCraft = resolve; }));
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Craft Wood Component" });
    button.click();
    await waitFor(() => expect(button).toBeDisabled());
    expect(craft).toHaveBeenCalledTimes(1);
    button.click();
    expect(craft).toHaveBeenCalledTimes(1);

    resolveCraft?.({ status: "success", ...campState, ap: 2990, inventory: [{ item: woodComponentRecipe.output, quantity: 1 }] });
    await waitFor(() => expect(screen.getByText("Wood Component: 1")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Craft Wood Component" })).toBeEnabled();
  });

  it("applies the authoritative state after an unsuccessful conversion", async () => {
    const stateWithWood = {
      ...campState,
      ap: 0,
      inventory: [{ item: { id: "wood", display_name: "Wood" }, quantity: 2 }],
      resources: resourcesWithWood(3),
    };
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...stateWithWood } });
    convert.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...stateWithWood });
    render(<App />);

    (await screen.findByRole("button", { name: "Convert" })).click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(apRow(0)).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Resources" })).toHaveTextContent("Wood: 3");
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

    resolveConvert?.({ status: "success", ...campState, ap: 2999, resources: resourcesWithWood(1) });
    await waitFor(() => expect(screen.getByRole("table", { name: "Resources" })).toHaveTextContent("Wood: 1"));
    expect(screen.getByRole("button", { name: "Convert" })).toBeEnabled();
  });

  it("applies the authoritative state after a successful move", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState } });
    move.mockResolvedValue({
      status: "success",
      location: { id: "forest_edge", display_name: "Forest edge" },
      routes: [{ origin_id: "forest_edge", destination_id: "camp", ap_cost: 20 }],
      ap: 2980,
      carried_weight: 0,
      movement_weight_threshold: 1000,
      inventory: [],
      ground_items: [],
      ground_resources: [],
      gathering_option: null,
      conversion_option: null,
      resources,
    });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Move to forest_edge" });
    button.click();
    await waitFor(() => expect(screen.getByText("Move succeeded. Current location: Forest edge")).toBeInTheDocument());
    expect(screen.getByText("Current location: Forest edge")).toBeInTheDocument();
    expect(screen.getByText("To camp (20 AP)")).toBeInTheDocument();
    expect(apRow(2980)).toBeInTheDocument();
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
    expect(apRow(10)).toBeInTheDocument();
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
      carried_weight: 0,
      movement_weight_threshold: 1000,
      inventory: [],
      ground_items: [],
      ground_resources: [],
      gathering_option: null,
      conversion_option: null,
      resources,
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
    expect(apRow(1)).toBeInTheDocument();
    expect(rest).toHaveBeenCalledTimes(1);
  });

  it("keeps the known AP and shows the rejection when AP is insufficient", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 0 } });
    rest.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ap: 0 });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(apRow(0)).toBeInTheDocument();
  });

  it("updates stale AP from an insufficient rest response", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 1 } });
    rest.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ap: 0 });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    expect(apRow(1)).toBeInTheDocument();
    button.click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(apRow(0)).toBeInTheDocument();
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
    await waitFor(() => expect(apRow(1)).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled();
  });

  it("keeps the known AP when rest fails", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ...campState, ap: 2 } });
    rest.mockResolvedValue({ status: "error", error: new Error("backend unavailable") });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("backend unavailable"));
    expect(apRow(2)).toBeInTheDocument();
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

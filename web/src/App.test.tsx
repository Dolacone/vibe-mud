import { cleanup, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import * as auth from "./auth";
import "./styles.css";

vi.mock("./auth", async () => {
  const actual = await vi.importActual<typeof import("./auth")>("./auth");
  return {
    ...actual,
    getCurrentUser: vi.fn(),
    rest: vi.fn(),
    move: vi.fn(),
    gather: vi.fn(),
    convert: vi.fn(),
    craft: vi.fn(),
    build: vi.fn(),
    contributeConstruction: vi.fn(),
    installExtension: vi.fn(),
    contributeExtensionConstruction: vi.fn(),
    removeExtension: vi.fn(),
    repairBuilding: vi.fn(),
    drop: vi.fn(),
    pickup: vi.fn(),
  };
});

const getCurrentUser = vi.mocked(auth.getCurrentUser);
const rest = vi.mocked(auth.rest);
const move = vi.mocked(auth.move);
const gather = vi.mocked(auth.gather);
const convert = vi.mocked(auth.convert);
const craft = vi.mocked(auth.craft);
const build = vi.mocked(auth.build);
const contributeConstruction = vi.mocked(auth.contributeConstruction);
const installExtension = vi.mocked(auth.installExtension);
const contributeExtensionConstruction = vi.mocked(auth.contributeExtensionConstruction);
const removeExtension = vi.mocked(auth.removeExtension);
const repairBuilding = vi.mocked(auth.repairBuilding);
const drop = vi.mocked(auth.drop);
const pickup = vi.mocked(auth.pickup);

const resources = ["food", "wood", "stone", "metal", "fiber", "hide", "medicinal", "arcane"].map((id) => ({
  resource: { id, display_name: id[0].toUpperCase() + id.slice(1) },
  quantity: 0,
}));

const resourcesWith = (id: string, quantity: number) => resources.map((entry) => entry.resource.id === id ? { ...entry, quantity } : entry);
const activeItem = (item: { id: string; display_name: string }, quantity: number) => ({ item, quantity, durability_status: "active" as const, durability_percentage: 100 });
const expiredItem = (item: { id: string; display_name: string }, quantity: number) => ({ item, quantity, durability_status: "expired" as const, durability_percentage: 0 });

const woodComponentRecipe = {
  id: "wood_component",
  display_name: "Wood Component",
  base_ap_cost: 10,
  resource_inputs: [{ resource: { id: "wood", display_name: "Wood" }, quantity: 10 }],
  item_inputs: [],
  output: { id: "wood_component", display_name: "Wood Component" },
  output_quantity: 1,
};

const handWoodMethod = {
  id: "hand_wood_t1",
  display_name: "Hand Wood Convert",
  ap_cost: 1,
  input: { id: "wood", display_name: "Wood" },
  max_input_quantity: 1,
  output_resource: { id: "wood", display_name: "Wood" },
  resource_quantity_per_input: 1,
  essence_item: null,
  essence_chance_bps: 0,
  essence_quantity: 0,
  provider_extension_ids: [],
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
  extension_slot_count: 2,
  durability_status: null,
  durability_percentage: null,
  available_actions: ["contribute-construction"],
  extensions: [{
    id: 7,
    slot_index: 0,
    definition_id: "sawmill_t1",
    display_name: "Sawmill T1",
    tier: 1,
    required_ap: 30,
    contributed_ap: 12,
    status: "under_construction" as const,
    available_actions: ["contribute-extension-construction"],
  }],
};

const completedWithExtension = {
  id: 2,
  owner: { id: 1, display_name: "Ada" },
  recipe: { id: "building_lv1", display_name: "Building Lv1" },
  building_level: 1,
  status: "completed" as const,
  extension_slot_count: 2,
  durability_status: "active" as const,
  durability_percentage: 99,
  available_actions: ["repair-building"],
  extensions: [{
    id: 8,
    slot_index: 1,
    definition_id: "sawmill_t1",
    display_name: "Sawmill T1",
    tier: 1,
    status: "completed" as const,
    available_actions: ["remove-extension"],
  }],
};

const sawmillDefinition = {
  id: "sawmill_t1",
  display_name: "Sawmill T1",
  tier: 1,
  package_item: { id: "sawmill_package_t1", display_name: "Sawmill Package T1" },
  required_ap: 30,
  installation_targets: [{ building_id: 1, slot_index: 1 }, { building_id: 2, slot_index: 0 }],
};

const sawmillMethod = {
  id: "sawmill_wood_t1",
  display_name: "Sawmill Wood Convert",
  ap_cost: 30,
  input: { id: "wood", display_name: "Wood" },
  max_input_quantity: 6,
  output_resource: { id: "wood", display_name: "Wood" },
  resource_quantity_per_input: 2,
  essence_item: null,
  essence_chance_bps: 1000,
  essence_quantity: 1,
  provider_extension_ids: [8],
};

const campState: auth.PlayerState = {
  available_actions: ["rest", "move", "convert", "craft", "build"],
  location: { id: "camp", display_name: "Camp" },
  routes: [{ origin_id: "camp", destination_id: "forest_edge", ap_cost: 20 }],
  ap: 3000,
  carried_weight: 0,
  movement_weight_threshold: 1000,
  inventory: [],
  ground_items: [],
  ground_resources: [],
  gathering_option: null,
  conversion_option: { item: { id: "wood", display_name: "Wood" }, resource: { id: "wood", display_name: "Wood" }, input_quantity: 1, resource_yield: 1, ap_cost: 1 },
  conversion_methods: [handWoodMethod],
  building_extension_definitions: [],
  resources,
  crafting_recipes: [woodComponentRecipe],
  building_recipes: [buildingRecipe],
  buildings: [],
};

const forestState: auth.PlayerState = {
  ...campState,
  available_actions: ["rest", "move", "gather", "craft", "build"],
  location: { id: "forest_edge", display_name: "Forest edge" },
  routes: [{ origin_id: "forest_edge", destination_id: "camp", ap_cost: 20 }],
  gathering_option: { item: { id: "wood", display_name: "Wood" }, quantity: 1, ap_cost: 10 },
  conversion_option: null,
  conversion_methods: [],
};

const authenticated = (state: auth.PlayerState = campState, displayName = "Ada"): auth.AuthResult => ({ status: "authenticated", user: { id: 1, display_name: displayName, email: "ada@example.com", ...state } });
const renderAuthenticated = async (state: auth.PlayerState = campState, displayName = "Ada") => {
  getCurrentUser.mockResolvedValue(authenticated(state, displayName));
  render(<App />);
  await screen.findByRole("heading", { name: "地圖" });
};
const selectTab = async (name: "地圖" | "地區" | "道具" | "角色") => {
  fireEvent.click(screen.getByRole("button", { name }));
  await screen.findByRole("heading", { name });
};

describe("App gameplay tab integration", () => {
  beforeEach(() => {
    getCurrentUser.mockReset();
    rest.mockReset();
    move.mockReset();
    gather.mockReset();
    convert.mockReset();
    craft.mockReset();
    build.mockReset();
    contributeConstruction.mockReset();
    installExtension.mockReset();
    contributeExtensionConstruction.mockReset();
    removeExtension.mockReset();
    repairBuilding.mockReset();
    drop.mockReset();
    pickup.mockReset();
  });

  it("keeps the authoritative header and four-tab navigation mounted", async () => {
    await renderAuthenticated();

    expect(screen.getByLabelText("玩家名稱 Ada")).toBeInTheDocument();
    expect(screen.getByLabelText("目前 AP 3000")).toBeInTheDocument();
    expect(screen.getByLabelText("目前 HP 尚未實作")).toHaveTextContent("HP --");
    expect(within(screen.getByRole("navigation", { name: "主分頁" })).getAllByRole("button")).toHaveLength(4);
    expect(screen.getByRole("button", { name: "地圖" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("table", { name: "Available routes" })).toHaveTextContent("To forest_edge (20 AP)");
    expect(screen.getByRole("table", { name: "Movement weight" })).toHaveTextContent("Carrying weight");
    expect(screen.getByRole("table", { name: "Movement weight" })).toHaveTextContent("Movement weight threshold");
  });

  it("assigns gathering, buildings, installation targets, and ground pickup to Area", async () => {
    const areaState = {
      ...forestState,
      available_actions: ["gather", "build", "install-extension"],
      building_extension_definitions: [sawmillDefinition],
      buildings: [underConstruction, completedWithExtension],
      ground_items: [activeItem({ id: "wood_component", display_name: "Wood Component" }, 2), expiredItem({ id: "wood_component", display_name: "Wood Component" }, 5)],
      ground_resources: [{ resource: { id: "stone", display_name: "Stone" }, quantity: 3 }],
    };
    await renderAuthenticated(areaState);
    await selectTab("地區");

    expect(screen.getByRole("table", { name: "Gather" })).toHaveTextContent("Yield: 1 Wood; Cost: 10 AP");
    expect(screen.getByRole("table", { name: "Building recipes" })).toHaveTextContent("Building Lv1");
    expect(screen.getByRole("table", { name: "Building extension definitions" })).toHaveTextContent("Installation targets");
    expect(screen.getByRole("button", { name: "Install in building 1, slot 1" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Install in building 2, slot 0" })).toBeEnabled();
    expect(screen.getByRole("table", { name: "Buildings" })).toHaveTextContent("Progress: 0/60 AP (0%)");
    expect(screen.getByRole("table", { name: "Buildings" })).toHaveTextContent("Sawmill T1: under_construction 12/30 AP (40%)");
    expect(screen.getByRole("button", { name: "Contribute AP to building 1" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Contribute AP to building 7" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Remove extension" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Repair building 2" })).toBeEnabled();
    expect(screen.getByRole("table", { name: "Ground Items" })).toHaveTextContent("Wood Component");
    expect(screen.getByRole("button", { name: "Pickup Wood Component" })).toBeEnabled();
    expect(screen.getByRole("table", { name: "Ground Resources" })).toHaveTextContent("Stone");
    expect(screen.getByRole("button", { name: "Pickup Stone" })).toBeEnabled();
  });

  it("assigns Inventory, eight Resources, Drops, provider Convert, and Craft to Items", async () => {
    const itemState = {
      ...campState,
      available_actions: ["convert", "craft"],
      inventory: [activeItem({ id: "wood", display_name: "Wood" }, 4)],
      resources: resourcesWith("wood", 6),
      conversion_methods: [handWoodMethod, sawmillMethod],
      buildings: [completedWithExtension],
    };
    await renderAuthenticated(itemState);
    await selectTab("道具");

    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Wood: 4");
    expect(screen.getByRole("table", { name: "Resources" }).querySelectorAll("tbody > tr")).toHaveLength(8);
    expect(screen.getByRole("table", { name: "Resources" })).toHaveTextContent("Wood: 6");
    expect(screen.getByRole("table", { name: "Convert" })).toHaveTextContent("Hand Wood Convert");
    expect(screen.getByRole("table", { name: "Convert" })).toHaveTextContent("Provider extension IDs: 8");
    expect(screen.getByRole("combobox", { name: "Provider for Sawmill Wood Convert" })).toHaveTextContent("Sawmill T1");
    expect(screen.getByRole("table", { name: "Craft" })).toHaveTextContent("Wood Component");
    expect(within(screen.getByRole("table", { name: "Inventory" })).getByRole("button", { name: "Drop Wood" })).toBeEnabled();
    expect(within(screen.getByRole("table", { name: "Inventory" })).queryByRole("button", { name: "Drop Wood (expired)" })).not.toBeInTheDocument();
  });

  it("keeps Rest, identity, and explicit progression placeholders in Character", async () => {
    await renderAuthenticated();
    await selectTab("角色");

    expect(screen.getByRole("table", { name: "Character identity" })).toHaveTextContent("ada@example.com");
    expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled();
    expect(screen.getByRole("table", { name: "Progression" })).toHaveTextContent("Equipment");
    expect(screen.getByRole("table", { name: "Progression" })).toHaveTextContent("Skills");
    expect(screen.getByRole("table", { name: "Progression" })).toHaveTextContent("Level");
    expect(screen.getByRole("table", { name: "Progression" })).toHaveTextContent("Not implemented");
  });

  it("never reconstructs gameplay controls when global availability omits them", async () => {
    const filtered = { ...campState, available_actions: ["rest"], gathering_option: { item: { id: "wood", display_name: "Wood" }, quantity: 1, ap_cost: 10 }, building_extension_definitions: [sawmillDefinition] };
    await renderAuthenticated(filtered);

    expect(screen.getByRole("table", { name: "Available routes" })).toHaveTextContent("No available routes.");
    await selectTab("地區");
    expect(screen.getByRole("table", { name: "Gather" })).toHaveTextContent("No gathering action available.");
    expect(screen.getByRole("table", { name: "Building recipes" })).toHaveTextContent("No building recipes available.");
    expect(screen.queryByRole("table", { name: "Building extension definitions" })).not.toBeInTheDocument();
    await selectTab("道具");
    expect(screen.getByRole("table", { name: "Convert" })).toHaveTextContent("No conversion action available.");
    expect(screen.getByRole("table", { name: "Craft" })).toHaveTextContent("No crafting recipes available.");
    await selectTab("角色");
    expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled();
  });

  it("moves from the Route list and applies authoritative state without changing tabs", async () => {
    await renderAuthenticated();
    move.mockResolvedValue({ ...forestState, status: "success", ap: 2980 });

    fireEvent.click(screen.getByRole("button", { name: "Move to forest_edge" }));
    await waitFor(() => expect(screen.getByText("Move succeeded. Current location: Forest edge")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "地圖" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByLabelText("目前 AP 2980")).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Available routes" })).toHaveTextContent("To camp (20 AP)");
    expect(move).toHaveBeenCalledWith("forest_edge");
  });

  it("shows carrying weight and the backend-filtered Route list", async () => {
    await renderAuthenticated({ ...campState, carried_weight: 1001, routes: [], available_actions: ["rest", "convert", "craft", "build"] });

    expect(screen.getByRole("table", { name: "Movement weight" })).toHaveTextContent("1001");
    expect(screen.getByRole("table", { name: "Movement weight" })).toHaveTextContent("1000");
    expect(screen.getByRole("alert")).toHaveTextContent("Cannot move while overweight.");
    expect(screen.queryByRole("button", { name: /Move to/ })).not.toBeInTheDocument();
  });

  it("keeps the shell mounted for request and domain failures", async () => {
    await renderAuthenticated();
    move.mockRejectedValue(new Error("backend unavailable"));
    fireEvent.click(screen.getByRole("button", { name: "Move to forest_edge" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("backend unavailable"));
    expect(screen.getByRole("navigation", { name: "主分頁" })).toBeInTheDocument();
    expect(screen.getByLabelText("玩家名稱 Ada")).toBeInTheDocument();

    await selectTab("角色");
    rest.mockRejectedValue(new Error("rest rejected"));
    fireEvent.click(screen.getByRole("button", { name: "Rest" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("rest rejected"));
    expect(screen.getByRole("navigation", { name: "主分頁" })).toBeInTheDocument();
    expect(screen.getByLabelText("目前 AP 3000")).toBeInTheDocument();
  });

  it("moves all Area building controls using backend action metadata", async () => {
    const areaState = { ...forestState, available_actions: ["build", "install-extension"], building_extension_definitions: [sawmillDefinition], buildings: [underConstruction, completedWithExtension] };
    await renderAuthenticated(areaState);
    await selectTab("地區");
    build.mockResolvedValue({ status: "success", ...areaState });
    installExtension.mockResolvedValue({ status: "success", ...areaState });
    contributeConstruction.mockResolvedValue({ status: "success", ...areaState });
    contributeExtensionConstruction.mockResolvedValue({ status: "success", ...areaState });
    removeExtension.mockResolvedValue({ status: "success", ...areaState });
    repairBuilding.mockResolvedValue({ status: "success", ...areaState });

    fireEvent.click(screen.getByRole("button", { name: "Build Building Lv1" }));
    await waitFor(() => expect(build).toHaveBeenCalledWith("building_lv1"));
    fireEvent.click(screen.getByRole("button", { name: "Install in building 1, slot 1" }));
    await waitFor(() => expect(installExtension).toHaveBeenCalledWith({ building_id: 1, slot_index: 1, definition_id: "sawmill_t1" }));
    const buildingInput = screen.getByRole("spinbutton", { name: "Contribution AP for building 1" });
    fireEvent.change(buildingInput, { target: { value: "15" } });
    fireEvent.submit(buildingInput.closest("form")!);
    await waitFor(() => expect(contributeConstruction).toHaveBeenCalledWith(1, 15));
    const extensionInput = screen.getByRole("spinbutton", { name: "Contribution AP for building 7" });
    fireEvent.change(extensionInput, { target: { value: "10" } });
    fireEvent.submit(extensionInput.closest("form")!);
    await waitFor(() => expect(contributeExtensionConstruction).toHaveBeenCalledWith(7, 10));
    fireEvent.click(screen.getByRole("button", { name: "Remove extension" }));
    await waitFor(() => expect(removeExtension).toHaveBeenCalledWith(8));
    fireEvent.click(screen.getByRole("button", { name: "Repair building 2" }));
    await waitFor(() => expect(repairBuilding).toHaveBeenCalledWith(2));
  });

  it("uses Item durability rules for Drop and Pickup", async () => {
    const transferState: auth.PlayerState = { ...campState, inventory: [activeItem({ id: "wood", display_name: "Wood" }, 4), expiredItem({ id: "wood", display_name: "Wood" }, 3)], ground_items: [activeItem({ id: "wood_component", display_name: "Wood Component" }, 2), expiredItem({ id: "wood_component", display_name: "Wood Component" }, 5)], ground_resources: [{ resource: { id: "stone", display_name: "Stone" }, quantity: 3 }] };
    await renderAuthenticated(transferState);
    await selectTab("道具");
    const inventory = screen.getByRole("table", { name: "Inventory" });
    expect(within(inventory).getByRole("button", { name: "Drop Wood" })).toBeEnabled();
    expect(within(inventory).getByRole("button", { name: "Drop Wood (expired)" })).toBeEnabled();
    const dropInput = within(inventory).getByRole("spinbutton", { name: "Drop quantity for Wood" });
    fireEvent.change(dropInput, { target: { value: "2" } });
    drop.mockResolvedValue({ status: "success", ...transferState });
    fireEvent.submit(dropInput.closest("form")!);
    await waitFor(() => expect(drop).toHaveBeenCalledWith({ asset_type: "item", asset_id: "wood", quantity: 2, item_status: "active" }));

    await selectTab("地區");
    expect(screen.getByRole("button", { name: "Pickup Wood Component" })).toBeEnabled();
    expect(screen.queryByRole("button", { name: "Pickup Wood Component (expired)" })).not.toBeInTheDocument();
    const pickupInput = screen.getByRole("spinbutton", { name: "Pickup quantity for Wood Component" });
    pickup.mockResolvedValue({ status: "success", ...transferState });
    fireEvent.submit(pickupInput.closest("form")!);
    await waitFor(() => expect(pickup).toHaveBeenCalledWith({ asset_type: "item", asset_id: "wood_component", quantity: 1, item_status: "active" }));
  });

  it("sends provider extension IDs and recipe IDs from backend metadata", async () => {
    const itemState = { ...campState, available_actions: ["convert", "craft"], conversion_methods: [sawmillMethod], buildings: [completedWithExtension], resources: resourcesWith("wood", 6) };
    await renderAuthenticated(itemState);
    await selectTab("道具");
    const provider = screen.getByRole("combobox", { name: "Provider for Sawmill Wood Convert" });
    fireEvent.change(provider, { target: { value: "8" } });
    convert.mockResolvedValue({ status: "success", ...itemState, ap: 2970, resources: resourcesWith("wood", 4) });
    fireEvent.click(screen.getByRole("button", { name: "Convert" }));
    await waitFor(() => expect(convert).toHaveBeenCalledWith("sawmill_wood_t1", 1, 8));
    craft.mockResolvedValue({ status: "success", ...itemState, ap: 2960 });
    fireEvent.click(screen.getByRole("button", { name: "Craft Wood Component" }));
    await waitFor(() => expect(craft).toHaveBeenCalledWith("wood_component"));
  });

  it("keeps the active tab while a successful action updates the header", async () => {
    await renderAuthenticated({ ...campState, available_actions: ["craft"], resources: resourcesWith("wood", 10) });
    await selectTab("道具");
    craft.mockResolvedValue({ status: "success", ...campState, available_actions: ["craft"], ap: 2990, resources: resourcesWith("wood", 0), inventory: [activeItem(woodComponentRecipe.output, 1)] });
    fireEvent.click(screen.getByRole("button", { name: "Craft Wood Component" }));
    await waitFor(() => expect(screen.getByText("Craft succeeded.")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "道具" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByLabelText("目前 AP 2990")).toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Wood Component: 1");
  });

  it("applies authoritative success and failure state in the active tab", async () => {
    await renderAuthenticated({ ...forestState, available_actions: ["gather"], ap: 2, resources: resourcesWith("wood", 1) });
    await selectTab("地區");
    gather.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...forestState, available_actions: ["gather"], ap: 0, resources: resourcesWith("wood", 1) });
    fireEvent.click(screen.getByRole("button", { name: "Gather" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("Gather failed: insufficient action points"));
    expect(screen.getByLabelText("目前 AP 0")).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "主分頁" })).toBeInTheDocument();
  });

  it("preserves signed-out and identity-error states", async () => {
    getCurrentUser.mockResolvedValue({ status: "unauthenticated" });
    render(<App />);
    await waitFor(() => expect(screen.getByRole("heading", { name: "Not signed in" })).toBeInTheDocument());
    expect(screen.getByRole("link", { name: /google/i })).toHaveAttribute("href", "/auth/google/login");

    getCurrentUser.mockResolvedValue({ status: "error", error: new Error("backend unavailable") });
    cleanup();
    const view = render(<App />);
    await waitFor(() => expect(view.getByRole("alert")).toHaveTextContent("backend unavailable"));
    expect(view.getByRole("link", { name: /google/i })).toBeInTheDocument();
  });
});

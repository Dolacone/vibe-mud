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

const completedActive = {
  ...completedWithExtension,
  id: 1,
  extensions: [],
};

const completedDisabled = {
  ...completedActive,
  durability_status: "disabled" as const,
  durability_percentage: 0,
};

const underConstructionSawmill = {
  id: 7,
  slot_index: 0,
  definition_id: "sawmill_t1",
  display_name: "Sawmill T1",
  tier: 1,
  required_ap: 30,
  contributed_ap: 12,
  status: "under_construction" as const,
  available_actions: ["contribute-extension-construction"],
};

const completedSawmill = {
  id: 8,
  slot_index: 1,
  definition_id: "sawmill_t1",
  display_name: "Sawmill T1",
  tier: 1,
  status: "completed" as const,
  available_actions: [],
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

const transferState: auth.PlayerState = {
  ...campState,
  inventory: [activeItem({ id: "wood", display_name: "Wood" }, 4)],
  resources: resourcesWith("wood", 6),
  ground_items: [activeItem({ id: "wood_component", display_name: "Wood Component" }, 2)],
  ground_resources: [{ resource: { id: "stone", display_name: "Stone" }, quantity: 3 }],
};

const allGameplayState: auth.PlayerState = {
  ...forestState,
  available_actions: ["rest", "move", "gather", "convert", "craft", "build"],
  conversion_option: null,
  conversion_methods: [handWoodMethod],
  inventory: [activeItem({ id: "wood", display_name: "Wood" }, 4)],
  ground_items: [activeItem({ id: "wood_component", display_name: "Wood Component" }, 2)],
  ground_resources: [{ resource: { id: "stone", display_name: "Stone" }, quantity: 3 }],
  buildings: [underConstruction, completedWithExtension],
};

const authenticated = (state: auth.PlayerState = campState, displayName = "Ada"): auth.AuthResult => ({ status: "authenticated", user: { id: 1, display_name: displayName, email: "ada@example.com", player_name: null, ...state } });
const renderAuthenticated = async (state: auth.PlayerState = campState, displayName = "Ada") => {
  getCurrentUser.mockResolvedValue(authenticated(state, displayName));
  render(<App />);
  await screen.findByRole("heading", { name: "地圖" });
};
const selectTab = async (name: "地圖" | "地區" | "道具" | "角色") => {
  fireEvent.click(screen.getByRole("button", { name }));
  await screen.findByRole("heading", { name });
};
const gameplayControls = () => [
  ...screen.getAllByRole("button", { hidden: true }).filter((button) => !button.closest("nav")),
  ...screen.getAllByRole("spinbutton", { hidden: true }),
  ...screen.queryAllByRole("combobox", { hidden: true }),
];
const navigationButtons = () => screen.getAllByRole("button", { hidden: true }).filter((button) => button.closest("nav"));

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

    expect(screen.getByLabelText("目前 AP 3000")).toBeInTheDocument();
    expect(screen.getByLabelText("目前 HP 尚未實作")).toHaveTextContent("HP --");
    expect(screen.getByLabelText("Weight 0/1000")).toBeInTheDocument();
    expect(within(screen.getByRole("navigation", { name: "主分頁" })).getAllByRole("button")).toHaveLength(4);
    expect(screen.getByRole("button", { name: "地圖" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByRole("table", { name: "Available routes" })).toHaveTextContent("To forest_edge (20 AP)");
    expect(screen.queryByRole("table", { name: "Movement weight" })).not.toBeInTheDocument();
  });

  it("contains every gameplay table in a scroll wrapper with scoped column headers", async () => {
    const tableCoverageState: auth.PlayerState = {
      ...allGameplayState,
      available_actions: [...allGameplayState.available_actions, "install-extension"],
      building_extension_definitions: [sawmillDefinition],
    };
    await renderAuthenticated(tableCoverageState);

    for (const tab of ["地圖", "地區", "道具", "角色"] as const) {
      await selectTab(tab);
      const tables = within(screen.getByRole("main", { name: tab })).getAllByRole("table");
      expect(tables.length).toBeGreaterThan(0);
      for (const table of tables) {
        expect(table.parentElement).toHaveClass("table-scroll");
        const columnHeaders = table.querySelectorAll("thead th");
        expect([...columnHeaders].every((header) => header.getAttribute("scope") === "col")).toBe(true);
      }
    }
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
    expect(screen.queryByRole("table", { name: "Resources" })).not.toBeInTheDocument();
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

  it("applies authoritative weight updates in the fixed header", async () => {
    await renderAuthenticated();
    move.mockResolvedValue({ ...forestState, status: "success", ap: 2980, carried_weight: 751, movement_weight_threshold: 1000 });

    fireEvent.click(screen.getByRole("button", { name: "Move to forest_edge" }));
    await waitFor(() => expect(screen.getByLabelText("Weight 751/1000")).toBeInTheDocument());
  });

  it("shows carrying weight and the backend-filtered Route list", async () => {
    await renderAuthenticated({ ...campState, carried_weight: 1001, routes: [], available_actions: ["rest", "convert", "craft", "build"] });

    expect(screen.getByLabelText("Weight 1001/1000")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Move to/ })).not.toBeInTheDocument();
  });

  it("allows movement when carrying weight equals the threshold", async () => {
    await renderAuthenticated({ ...campState, carried_weight: campState.movement_weight_threshold });

    expect(screen.getByRole("button", { name: "Move to forest_edge" })).toBeEnabled();
    expect(screen.queryByText("Cannot move while overweight.")).not.toBeInTheDocument();
  });

  it("keeps the shell mounted for request and domain failures", async () => {
    await renderAuthenticated();
    move.mockRejectedValue(new Error("backend unavailable"));
    fireEvent.click(screen.getByRole("button", { name: "Move to forest_edge" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("backend unavailable"));
    expect(screen.getByRole("navigation", { name: "主分頁" })).toBeInTheDocument();

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
    const inventoryRows = inventory.querySelectorAll("tbody > tr");
    expect(inventoryRows[0]).toHaveTextContent("Status: active");
    expect(inventoryRows[0]).toHaveTextContent("Durability: 100%");
    expect(inventoryRows[1]).toHaveTextContent("Status: expired");
    expect(inventoryRows[1]).toHaveTextContent("Durability: 0%");
    const dropInput = within(inventory).getByRole("spinbutton", { name: "Drop quantity for Wood" });
    fireEvent.change(dropInput, { target: { value: "2" } });
    drop.mockResolvedValue({ status: "success", ...transferState });
    fireEvent.submit(dropInput.closest("form")!);
    await waitFor(() => expect(drop).toHaveBeenCalledWith({ asset_type: "item", asset_id: "wood", quantity: 2, item_status: "active" }));

    await selectTab("地區");
    const groundItemRows = screen.getByRole("table", { name: "Ground Items" }).querySelectorAll("tbody > tr");
    expect(groundItemRows[0]).toHaveTextContent("Status: active");
    expect(groundItemRows[0]).toHaveTextContent("Durability: 100%");
    expect(groundItemRows[1]).toHaveTextContent("Status: expired");
    expect(groundItemRows[1]).toHaveTextContent("Durability: 0%");
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

  it("keeps empty gameplay sections as explicit rows in their owning tabs", async () => {
    await renderAuthenticated({ ...campState, routes: [], gathering_option: null, conversion_option: null, conversion_methods: [], crafting_recipes: [], building_recipes: [], buildings: [], ground_items: [], ground_resources: [] });

    expect(screen.getByRole("table", { name: "Available routes" })).toHaveTextContent("No available routes.");
    await selectTab("地區");
    expect(screen.getByRole("table", { name: "Gather" })).toHaveTextContent("No gathering action available.");
    expect(screen.getByRole("table", { name: "Building recipes" })).toHaveTextContent("No building recipes available.");
    expect(screen.getByRole("table", { name: "Buildings" })).toHaveTextContent("No buildings at this location.");
    expect(screen.getByRole("table", { name: "Ground Items" })).toHaveTextContent("Ground items are empty.");
    expect(screen.getByRole("table", { name: "Ground Resources" })).toHaveTextContent("Ground resources are empty.");
    await selectTab("道具");
    expect(screen.getByRole("table", { name: "Convert" })).toHaveTextContent("No conversion action available.");
    expect(screen.getByRole("table", { name: "Craft" })).toHaveTextContent("No crafting recipes available.");
    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Inventory is empty.");
  });

  it("renders and submits a backend-returned legacy conversion in Items", async () => {
    const legacyState = { ...campState, conversion_methods: [] };
    await renderAuthenticated(legacyState);
    await selectTab("道具");
    convert.mockResolvedValue({ status: "success", ...legacyState, ap: 2999 });

    const convertTable = screen.getByRole("table", { name: "Convert" });
    expect(convertTable).toHaveTextContent("1 Wood");
    fireEvent.click(within(convertTable).getByRole("button", { name: "Convert" }));
    await waitFor(() => expect(screen.getByText("Convert succeeded.")).toBeInTheDocument());
    expect(convert).toHaveBeenCalledWith(fetch);
  });

  it("renders public ground items and resources with Area pickup controls", async () => {
    await renderAuthenticated({ ...campState, ground_items: [activeItem({ id: "wood_component", display_name: "Wood Component" }, 2)], ground_resources: [{ resource: { id: "stone", display_name: "Stone" }, quantity: 3 }] });
    await selectTab("地區");

    const groundItems = screen.getByRole("table", { name: "Ground Items" });
    const groundResources = screen.getByRole("table", { name: "Ground Resources" });
    expect(groundItems).toHaveTextContent("Wood Component");
    expect(within(groundItems).getByRole("spinbutton", { name: "Pickup quantity for Wood Component" })).toHaveValue(1);
    expect(within(groundItems).getByRole("button", { name: "Pickup Wood Component" })).toBeEnabled();
    expect(groundResources).toHaveTextContent("Stone");
    expect(within(groundResources).getByRole("spinbutton", { name: "Pickup quantity for Stone" })).toHaveValue(1);
    expect(within(groundResources).getByRole("button", { name: "Pickup Stone" })).toBeEnabled();
  });

  it("drops an Item with the requested quantity and applies authoritative state", async () => {
    const nextState = { ...transferState, inventory: [activeItem({ id: "wood", display_name: "Wood" }, 2)], ground_items: [activeItem({ id: "wood_component", display_name: "Wood Component" }, 4)] };
    await renderAuthenticated(transferState);
    await selectTab("道具");
    drop.mockResolvedValue({ status: "success", ...nextState });

    const inventory = screen.getByRole("table", { name: "Inventory" });
    const input = within(inventory).getByRole("spinbutton", { name: "Drop quantity for Wood" });
    fireEvent.change(input, { target: { value: "2" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByText("Drop succeeded.")).toBeInTheDocument());
    expect(drop).toHaveBeenCalledWith({ asset_type: "item", asset_id: "wood", quantity: 2, item_status: "active" });
    expect(within(inventory).getByText("Wood: 2")).toBeInTheDocument();
    await selectTab("地區");
    expect(within(screen.getByRole("table", { name: "Ground Items" })).getByText("4")).toBeInTheDocument();
  });

  it("picks up an Item with the requested quantity", async () => {
    const nextState = { ...transferState, inventory: [activeItem({ id: "wood", display_name: "Wood" }, 5)], ground_items: [activeItem({ id: "wood_component", display_name: "Wood Component" }, 1)] };
    await renderAuthenticated(transferState);
    await selectTab("地區");
    pickup.mockResolvedValue({ status: "success", ...nextState });

    const groundItems = screen.getByRole("table", { name: "Ground Items" });
    const input = within(groundItems).getByRole("spinbutton", { name: "Pickup quantity for Wood Component" });
    fireEvent.change(input, { target: { value: "1" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByText("Pickup succeeded.")).toBeInTheDocument());
    expect(pickup).toHaveBeenCalledWith({ asset_type: "item", asset_id: "wood_component", quantity: 1, item_status: "active" });
    expect(within(groundItems).getByText("1")).toBeInTheDocument();
  });

  it("drops the selected expired stack and renders authoritative durability state", async () => {
    const mixedState = { ...transferState, inventory: [activeItem({ id: "wood", display_name: "Wood" }, 4), expiredItem({ id: "wood", display_name: "Wood" }, 3)], ground_items: [activeItem({ id: "wood_component", display_name: "Wood Component" }, 2), expiredItem({ id: "wood_component", display_name: "Wood Component" }, 5)] };
    const nextState = { ...mixedState, inventory: [activeItem({ id: "wood", display_name: "Wood" }, 4), expiredItem({ id: "wood", display_name: "Wood" }, 2)], ground_items: [activeItem({ id: "wood_component", display_name: "Wood Component" }, 2), expiredItem({ id: "wood_component", display_name: "Wood Component" }, 6)] };
    await renderAuthenticated(mixedState);
    await selectTab("道具");
    drop.mockResolvedValue({ status: "success", ...nextState });

    const inventory = screen.getByRole("table", { name: "Inventory" });
    const input = within(inventory).getByRole("spinbutton", { name: "Drop quantity for Wood (expired)" });
    fireEvent.change(input, { target: { value: "1" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByText("Drop succeeded.")).toBeInTheDocument());
    expect(drop).toHaveBeenCalledWith({ asset_type: "item", asset_id: "wood", quantity: 1, item_status: "expired" });
    expect(within(inventory).getByText("Wood: 2")).toBeInTheDocument();
    await selectTab("地區");
    expect(within(screen.getByRole("table", { name: "Ground Items" })).getByText("6")).toBeInTheDocument();
  });

  it("applies authoritative state after an unsuccessful Resource pickup", async () => {
    const nextState = { ...transferState, ap: 2998, resources: resourcesWith("wood", 8), ground_resources: [{ resource: { id: "stone", display_name: "Stone" }, quantity: 1 }] };
    await renderAuthenticated(transferState);
    await selectTab("地區");
    pickup.mockResolvedValue({ status: "conflict", error: "insufficient ground quantity", ...nextState });

    const groundResources = screen.getByRole("table", { name: "Ground Resources" });
    fireEvent.submit(within(groundResources).getByRole("spinbutton", { name: "Pickup quantity for Stone" }).closest("form")!);
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient ground quantity"));
    expect(pickup).toHaveBeenCalledWith({ asset_type: "resource", asset_id: "stone", quantity: 1 });
    expect(screen.getByLabelText("目前 AP 2998")).toBeInTheDocument();
    expect(within(groundResources).getByText("1")).toBeInTheDocument();
    await selectTab("道具");
    expect(screen.getByLabelText("Resource 持有量")).toHaveTextContent("Wood 8");
  });

  it("prevents duplicate transfers while one request is pending", async () => {
    let resolveDrop: ((value: auth.TransferResult) => void) | undefined;
    drop.mockReturnValue(new Promise((resolve) => { resolveDrop = resolve; }));
    await renderAuthenticated(transferState);
    await selectTab("道具");

    const inventory = screen.getByRole("table", { name: "Inventory" });
    const input = within(inventory).getByRole("spinbutton", { name: "Drop quantity for Wood" });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(within(inventory).getByRole("button", { name: "Drop Wood" })).toBeDisabled());
    fireEvent.submit(input.closest("form")!);
    expect(drop).toHaveBeenCalledTimes(1);
    resolveDrop?.({ status: "success", ...transferState });
    await waitFor(() => expect(screen.getByText("Drop succeeded.")).toBeInTheDocument());
  });

  it("shows active Building durability and a repair action in Area", async () => {
    await renderAuthenticated({ ...campState, buildings: [completedActive] });
    await selectTab("地區");

    expect(screen.getByText("Durability status: active")).toBeInTheDocument();
    expect(screen.getByText("Durability: 99%")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Repair building 1" })).toBeEnabled();
  });

  it("shows disabled Building durability with zero percent", async () => {
    await renderAuthenticated({ ...campState, buildings: [completedDisabled] });
    await selectTab("地區");

    expect(screen.getByText("Durability status: disabled")).toBeInTheDocument();
    expect(screen.getByText("Durability: 0%")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Repair building 1" })).toBeEnabled();
  });

  it("repairs a completed Building and applies authoritative state", async () => {
    await renderAuthenticated({ ...campState, buildings: [completedActive] });
    await selectTab("地區");
    repairBuilding.mockResolvedValue({ status: "success", ...campState, ap: 2990, buildings: [{ ...completedActive, durability_percentage: 100 }] });

    fireEvent.click(screen.getByRole("button", { name: "Repair building 1" }));
    await waitFor(() => expect(screen.getByText("Building repair succeeded.")).toBeInTheDocument());
    expect(repairBuilding).toHaveBeenCalledWith(1);
    expect(screen.getByLabelText("目前 AP 2990")).toBeInTheDocument();
    expect(screen.getByText("Durability: 100%")).toBeInTheDocument();
  });

  it("shows a repair failure and applies its authoritative state", async () => {
    await renderAuthenticated({ ...campState, buildings: [completedActive] });
    await selectTab("地區");
    repairBuilding.mockResolvedValue({ status: "conflict", error: "insufficient action points", ...campState, ap: 5, buildings: [completedDisabled] });

    fireEvent.click(screen.getByRole("button", { name: "Repair building 1" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByLabelText("目前 AP 5")).toBeInTheDocument();
    expect(screen.getByText("Durability status: disabled")).toBeInTheDocument();
    expect(screen.getByText("Durability: 0%")).toBeInTheDocument();
  });

  it("displays the backend building recipe and current-location construction state", async () => {
    await renderAuthenticated({ ...campState, buildings: [underConstruction] });
    await selectTab("地區");

    expect(screen.getByRole("table", { name: "Building recipes" })).toHaveTextContent("Building Lv1");
    expect(screen.getByRole("table", { name: "Building recipes" })).toHaveTextContent("Required AP: 60");
    expect(screen.getByText("Wood Component: 1")).toBeInTheDocument();
    expect(screen.getByText("Owner: Ada")).toBeInTheDocument();
    expect(screen.getByText("Status: under_construction")).toBeInTheDocument();
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
    expect(screen.getByText("Empty extension slots: 1")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Build Building Lv1" })).toBeEnabled();
  });

  it("shows construction AP only for in-progress Buildings and extensions", async () => {
    const building = { ...underConstruction, required_ap: 6, contributed_ap: 1, extensions: [{ ...underConstructionSawmill, required_ap: 6, contributed_ap: 1 }, completedSawmill] };
    await renderAuthenticated({ ...campState, available_actions: [...campState.available_actions, "install-extension"], conversion_methods: [sawmillMethod], building_extension_definitions: [sawmillDefinition], buildings: [building] });
    await selectTab("地區");

    const buildingsTable = screen.getByRole("table", { name: "Buildings" });
    expect(buildingsTable).toHaveTextContent("Progress: 1/6 AP (16%)");
    expect(buildingsTable).toHaveTextContent("Sawmill T1: under_construction 1/6 AP (16%)");
    expect(buildingsTable).toHaveTextContent("Sawmill T1: completed");
    expect(buildingsTable).not.toHaveTextContent("Sawmill T1: completed 30/30 AP");

    const definitionsTable = screen.getByRole("table", { name: "Building extension definitions" });
    expect(definitionsTable).toHaveTextContent("Sawmill T1");
    expect(definitionsTable).not.toHaveTextContent("Sawmill T1 T1");
    await selectTab("道具");
    const provider = within(screen.getByRole("table", { name: "Convert" })).getByRole("combobox", { name: "Provider for Sawmill Wood Convert" });
    expect(within(provider).getByRole("option", { name: "Sawmill T1" })).toBeInTheDocument();
    expect(within(provider).queryByRole("option", { name: "Sawmill T1 T1" })).not.toBeInTheDocument();
  });

  it("starts construction with only the backend recipe identifier and applies authoritative state", async () => {
    await renderAuthenticated(campState);
    await selectTab("地區");
    build.mockResolvedValue({ status: "success", ...campState, ap: 3000, inventory: [], buildings: [underConstruction] });

    fireEvent.click(screen.getByRole("button", { name: "Build Building Lv1" }));
    await waitFor(() => expect(screen.getByText("Building construction started.")).toBeInTheDocument());
    expect(build).toHaveBeenCalledWith("building_lv1");
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
  });

  it("restores the persisted building after a page reload", async () => {
    getCurrentUser.mockResolvedValue(authenticated({ ...campState, buildings: [underConstruction] }));
    const view = render(<App />);
    await screen.findByRole("heading", { name: "地圖" });
    await selectTab("地區");
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
    view.unmount();

    render(<App />);
    await screen.findByRole("heading", { name: "地圖" });
    await selectTab("地區");
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
  });

  it("shows an occupied-slot failure with authoritative state", async () => {
    await renderAuthenticated({ ...campState, buildings: [underConstruction] });
    await selectTab("地區");
    build.mockResolvedValue({ status: "invalid", error: "building already exists", ...campState, buildings: [underConstruction] });

    fireEvent.click(screen.getByRole("button", { name: "Build Building Lv1" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("building already exists"));
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
  });

  it("allows same-location contribution and caps oversized AP at completion", async () => {
    await renderAuthenticated({ ...campState, ap: 100, buildings: [underConstruction] });
    await selectTab("地區");
    contributeConstruction.mockResolvedValue({ status: "success", ...campState, ap: 40, buildings: [completedActive] });

    const input = screen.getByRole("spinbutton", { name: "Contribution AP for building 1" });
    fireEvent.change(input, { target: { value: "100" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByText("Construction contribution succeeded.")).toBeInTheDocument());
    expect(contributeConstruction).toHaveBeenCalledWith(1, 100);
    expect(screen.getByText("Status: completed")).toBeInTheDocument();
    expect(screen.queryByText("Progress: 60/60 AP (100%)")).not.toBeInTheDocument();
    expect(screen.getByRole("table", { name: "Buildings" })).not.toHaveTextContent("60/60 AP");
    expect(screen.queryByRole("spinbutton", { name: "Contribution AP for building 1" })).not.toBeInTheDocument();
  });

  it("prevents duplicate building actions while one request is pending", async () => {
    let resolveBuild: ((value: auth.BuildResult) => void) | undefined;
    build.mockReturnValue(new Promise((resolve) => { resolveBuild = resolve; }));
    await renderAuthenticated(campState);
    await selectTab("地區");

    const button = screen.getByRole("button", { name: "Build Building Lv1" });
    fireEvent.click(button);
    await waitFor(() => expect(button).toBeDisabled());
    fireEvent.click(button);
    expect(build).toHaveBeenCalledTimes(1);
    resolveBuild?.({ status: "success", ...campState, buildings: [underConstruction] });
    await waitFor(() => expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument());
  });

  it("keeps authoritative AP and progress after a failed contribution", async () => {
    await renderAuthenticated({ ...campState, ap: 5, buildings: [underConstruction] });
    await selectTab("地區");
    contributeConstruction.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...campState, ap: 5, buildings: [underConstruction] });

    const input = screen.getByRole("spinbutton", { name: "Contribution AP for building 1" });
    fireEvent.change(input, { target: { value: "10" } });
    fireEvent.submit(input.closest("form")!);
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByLabelText("目前 AP 5")).toBeInTheDocument();
    expect(screen.getByText("Progress: 0/60 AP (0%)")).toBeInTheDocument();
  });

  it("shows the backend gathering option and applies authoritative state after success", async () => {
    await renderAuthenticated(forestState);
    await selectTab("地區");
    gather.mockResolvedValue({ status: "success", ...forestState, ap: 2970, inventory: [activeItem({ id: "wood", display_name: "Oak wood" }, 7)], gathering_option: { item: { id: "wood", display_name: "Oak wood" }, quantity: 3, ap_cost: 12 } });

    expect(screen.getByText("Yield: 1 Wood; Cost: 10 AP")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Gather" }));
    await waitFor(() => expect(screen.getByText("Gather succeeded.")).toBeInTheDocument());
    expect(screen.getByLabelText("目前 AP 2970")).toBeInTheDocument();
    await selectTab("道具");
    expect(screen.getByText("Oak wood: 7")).toBeInTheDocument();
    await selectTab("地區");
    expect(screen.getByRole("table", { name: "Gather" })).toHaveTextContent("Yield: 3 Oak wood; Cost: 12 AP");
    expect(gather).toHaveBeenCalledTimes(1);
  });

  it("applies authoritative state after an insufficient gather", async () => {
    await renderAuthenticated({ ...forestState, ap: 1 });
    await selectTab("地區");
    gather.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...forestState, ap: 0, inventory: [activeItem({ id: "wood", display_name: "Wood" }, 4)] });

    fireEvent.click(screen.getByRole("button", { name: "Gather" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByLabelText("目前 AP 0")).toBeInTheDocument();
    await selectTab("道具");
    expect(screen.getByText("Wood: 4")).toBeInTheDocument();
  });

  it("disables the active action and prevents duplicate gathers while pending", async () => {
    let resolveGather: ((value: auth.GatherResult) => void) | undefined;
    gather.mockReturnValue(new Promise((resolve) => { resolveGather = resolve; }));
    await renderAuthenticated(forestState);
    await selectTab("地區");

    const gatherButton = screen.getByRole("button", { name: "Gather" });
    fireEvent.click(gatherButton);
    await waitFor(() => expect(gatherButton).toBeDisabled());
    fireEvent.click(gatherButton);
    expect(gather).toHaveBeenCalledTimes(1);
    resolveGather?.({ status: "success", ...forestState, ap: 2970, inventory: [activeItem({ id: "wood", display_name: "Wood" }, 1)] });
    await waitFor(() => expect(screen.getByText("Wood: 1")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Gather" })).toBeEnabled();
  });

  it("disables every gameplay control during pending Gather while keeping navigation enabled", async () => {
    let resolveGather: ((value: auth.GatherResult) => void) | undefined;
    gather.mockReturnValue(new Promise((resolve) => { resolveGather = resolve; }));
    await renderAuthenticated(allGameplayState);
    await selectTab("地區");

    fireEvent.click(screen.getByRole("button", { name: "Gather" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Gathering..." })).toBeDisabled());
    expect(gameplayControls().length).toBeGreaterThan(0);
    expect(gameplayControls().every((control) => control.hasAttribute("disabled"))).toBe(true);
    expect(navigationButtons().every((button) => !button.hasAttribute("disabled"))).toBe(true);

    await selectTab("道具");
    expect(screen.getByRole("button", { name: "Convert" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Craft Wood Component" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Converting..." })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Crafting..." })).not.toBeInTheDocument();
    await selectTab("地圖");
    expect(screen.getByRole("button", { name: "Move to camp" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Moving..." })).not.toBeInTheDocument();
    await selectTab("角色");
    expect(screen.getByRole("button", { name: "Rest" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Resting..." })).not.toBeInTheDocument();

    resolveGather?.({ status: "success", ...allGameplayState });
    await waitFor(() => expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled());
  });

  it("applies authoritative state after converting the last Wood", async () => {
    const stateWithWood = { ...campState, inventory: [activeItem({ id: "wood", display_name: "Wood" }, 1)] };
    await renderAuthenticated(stateWithWood);
    await selectTab("道具");
    convert.mockResolvedValue({ status: "success", ...campState, ap: 2999, resources: resourcesWith("wood", 1) });

    fireEvent.click(screen.getByRole("button", { name: "Convert" }));
    await waitFor(() => expect(screen.getByText("Convert succeeded.")).toBeInTheDocument());
    expect(screen.getByLabelText("目前 AP 2999")).toBeInTheDocument();
    expect(screen.getByLabelText("Resource 持有量")).toHaveTextContent("Wood 1");
    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Inventory is empty.");
    expect(convert).toHaveBeenCalledTimes(1);
  });

  it("crafts by recipe identifier and displays authoritative state", async () => {
    await renderAuthenticated({ ...campState, resources: resourcesWith("wood", 10) });
    await selectTab("道具");
    craft.mockResolvedValue({ status: "success", ...campState, ap: 2990, resources: resourcesWith("wood", 0), inventory: [activeItem(woodComponentRecipe.output, 1)] });

    fireEvent.click(screen.getByRole("button", { name: "Craft Wood Component" }));
    await waitFor(() => expect(screen.getByText("Craft succeeded.")).toBeInTheDocument());
    expect(screen.getByLabelText("目前 AP 2990")).toBeInTheDocument();
    expect(screen.queryByLabelText("Resource 持有量")).not.toHaveTextContent("Wood 0");
    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Wood Component: 1");
    expect(craft).toHaveBeenCalledWith("wood_component");
  });

  it("displays the recipe at a non-camp location", async () => {
    await renderAuthenticated(forestState);
    await selectTab("道具");

    expect(screen.getByRole("table", { name: "Craft" })).toHaveTextContent("Wood Component");
    await selectTab("地圖");
    expect(screen.getByText("Current location: Forest edge")).toBeInTheDocument();
  });

  it("keeps authoritative state after a failed craft", async () => {
    await renderAuthenticated({ ...campState, ap: 5, resources: resourcesWith("wood", 2) });
    await selectTab("道具");
    craft.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...campState, ap: 5, resources: resourcesWith("wood", 2) });

    fireEvent.click(screen.getByRole("button", { name: "Craft Wood Component" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByLabelText("目前 AP 5")).toBeInTheDocument();
    expect(screen.getByLabelText("Resource 持有量")).toHaveTextContent("Wood 2");
    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Inventory is empty.");
  });

  it("prevents duplicate crafts while one request is pending", async () => {
    let resolveCraft: ((value: auth.CraftResult) => void) | undefined;
    craft.mockReturnValue(new Promise((resolve) => { resolveCraft = resolve; }));
    await renderAuthenticated(campState);
    await selectTab("道具");

    const button = screen.getByRole("button", { name: "Craft Wood Component" });
    fireEvent.click(button);
    await waitFor(() => expect(button).toBeDisabled());
    fireEvent.click(button);
    expect(craft).toHaveBeenCalledTimes(1);
    resolveCraft?.({ status: "success", ...campState, ap: 2990, inventory: [activeItem(woodComponentRecipe.output, 1)] });
    await waitFor(() => expect(screen.getByText("Wood Component: 1")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Craft Wood Component" })).toBeEnabled();
  });

  it("applies authoritative state after an unsuccessful conversion", async () => {
    const stateWithWood = { ...campState, ap: 0, inventory: [activeItem({ id: "wood", display_name: "Wood" }, 2)], resources: resourcesWith("wood", 3) };
    await renderAuthenticated(stateWithWood);
    await selectTab("道具");
    convert.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...stateWithWood });

    fireEvent.click(screen.getByRole("button", { name: "Convert" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByLabelText("目前 AP 0")).toBeInTheDocument();
    expect(screen.getByLabelText("Resource 持有量")).toHaveTextContent("Wood 3");
    expect(screen.getByRole("table", { name: "Inventory" })).toHaveTextContent("Wood: 2");
  });

  it("disables the active action and prevents duplicate conversions while pending", async () => {
    let resolveConvert: ((value: auth.ConvertResult) => void) | undefined;
    convert.mockReturnValue(new Promise((resolve) => { resolveConvert = resolve; }));
    await renderAuthenticated(campState);
    await selectTab("道具");

    const button = screen.getByRole("button", { name: "Convert" });
    fireEvent.click(button);
    await waitFor(() => expect(button).toBeDisabled());
    fireEvent.click(button);
    expect(convert).toHaveBeenCalledTimes(1);
    resolveConvert?.({ status: "success", ...campState, ap: 2999, resources: resourcesWith("wood", 1) });
    await waitFor(() => expect(screen.getByLabelText("Resource 持有量")).toHaveTextContent("Wood 1"));
    expect(screen.getByRole("button", { name: "Convert" })).toBeEnabled();
  });

  it("disables every gameplay control during pending Convert while keeping navigation enabled", async () => {
    let resolveConvert: ((value: auth.ConvertResult) => void) | undefined;
    convert.mockReturnValue(new Promise((resolve) => { resolveConvert = resolve; }));
    await renderAuthenticated(allGameplayState);
    await selectTab("道具");

    fireEvent.click(screen.getByRole("button", { name: "Convert" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Converting..." })).toBeDisabled());
    expect(gameplayControls().length).toBeGreaterThan(0);
    expect(gameplayControls().every((control) => control.hasAttribute("disabled"))).toBe(true);
    expect(navigationButtons().every((button) => !button.hasAttribute("disabled"))).toBe(true);

    await selectTab("地區");
    expect(screen.getByRole("button", { name: "Gather" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Gathering..." })).not.toBeInTheDocument();
    await selectTab("地圖");
    expect(screen.getByRole("button", { name: "Move to camp" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Moving..." })).not.toBeInTheDocument();
    await selectTab("角色");
    expect(screen.getByRole("button", { name: "Rest" })).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Resting..." })).not.toBeInTheDocument();

    resolveConvert?.({ status: "success", ...allGameplayState });
    await waitFor(() => expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled());
  });

  it("keeps the authoritative state after an insufficient move", async () => {
    await renderAuthenticated({ ...campState, ap: 10 });
    move.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ...campState, ap: 10 });

    const button = screen.getByRole("button", { name: "Move to forest_edge" });
    fireEvent.click(button);
    await waitFor(() => expect(screen.getByText("Move failed: insufficient action points")).toBeInTheDocument());
    expect(screen.getByText("Current location: Camp")).toBeInTheDocument();
    expect(screen.getByLabelText("目前 AP 10")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Move to forest_edge" })).toBeEnabled();
  });

  it("prevents duplicate move requests while one request is pending", async () => {
    let resolveMove: ((value: auth.MoveResult) => void) | undefined;
    move.mockReturnValue(new Promise((resolve) => { resolveMove = resolve; }));
    await renderAuthenticated(campState);

    const button = screen.getByRole("button", { name: "Move to forest_edge" });
    fireEvent.click(button);
    await waitFor(() => expect(button).toBeDisabled());
    fireEvent.click(button);
    expect(move).toHaveBeenCalledTimes(1);
    resolveMove?.({ ...forestState, status: "success", ap: 2980 });
    await waitFor(() => expect(screen.getByText("Current location: Forest edge")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Move to camp" })).toBeEnabled();
  });

  it("updates the displayed AP after a successful rest", async () => {
    await renderAuthenticated({ ...campState, ap: 2 });
    await selectTab("角色");
    rest.mockResolvedValue({ ...campState, status: "success", ap: 1 });

    fireEvent.click(screen.getByRole("button", { name: "Rest" }));
    await waitFor(() => expect(screen.getByText("Rest succeeded. AP: 1")).toBeInTheDocument());
    expect(screen.getByLabelText("目前 AP 1")).toBeInTheDocument();
    expect(rest).toHaveBeenCalledTimes(1);
  });

  it("hides Rest when the backend does not return it", async () => {
    await renderAuthenticated({ ...campState, ap: 0, available_actions: ["move"] });
    await selectTab("角色");

    expect(screen.getByText("No actions available.")).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Rest" })).not.toBeInTheDocument();
    expect(screen.getByLabelText("目前 AP 0")).toBeInTheDocument();
    expect(rest).not.toHaveBeenCalled();
  });

  it("updates stale AP from an insufficient Rest response", async () => {
    await renderAuthenticated({ ...campState, ap: 1 });
    await selectTab("角色");
    rest.mockResolvedValue({ ...campState, status: "insufficient", error: "insufficient action points", ap: 0, available_actions: ["move"] });

    const button = screen.getByRole("button", { name: "Rest" });
    expect(screen.getByLabelText("目前 AP 1")).toBeInTheDocument();
    fireEvent.click(button);
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByLabelText("目前 AP 0")).toBeInTheDocument();
    expect(screen.queryByText("AP: 1")).not.toBeInTheDocument();
  });

  it("disables Rest and prevents duplicate requests while one is pending", async () => {
    let resolveRest: ((value: auth.RestResult) => void) | undefined;
    rest.mockReturnValue(new Promise((resolve) => { resolveRest = resolve; }));
    await renderAuthenticated({ ...campState, ap: 2 });
    await selectTab("角色");

    const button = screen.getByRole("button", { name: "Rest" });
    fireEvent.click(button);
    await waitFor(() => expect(button).toBeDisabled());
    fireEvent.click(button);
    expect(rest).toHaveBeenCalledTimes(1);
    resolveRest?.({ ...campState, status: "success", ap: 1 });
    await waitFor(() => expect(screen.getByLabelText("目前 AP 1")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled();
  });

  it("keeps the known AP when Rest fails", async () => {
    await renderAuthenticated({ ...campState, ap: 2 });
    await selectTab("角色");
    rest.mockRejectedValue(new Error("backend unavailable"));

    fireEvent.click(screen.getByRole("button", { name: "Rest" }));
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("backend unavailable"));
    expect(screen.getByLabelText("目前 AP 2")).toBeInTheDocument();
  });

  it("loads and displays only the backend-confirmed identity", async () => {
    getCurrentUser.mockResolvedValue(authenticated());
    render(<App />);

    expect(screen.getByRole("status")).toHaveTextContent("Loading");
    await screen.findByRole("heading", { name: "地圖" });
    await selectTab("角色");
    expect(screen.getByRole("table", { name: "Character identity" })).toHaveTextContent("1");
    expect(screen.getByRole("table", { name: "Character identity" })).toHaveTextContent("ada@example.com");
    expect(screen.getByLabelText("目前 AP 3000")).toBeInTheDocument();
    expect(screen.queryByText(/role|token/i)).not.toBeInTheDocument();
  });

  it("does not retain identity after an unauthenticated response", async () => {
    getCurrentUser.mockResolvedValue({ status: "unauthenticated" });
    render(<App />);

    await waitFor(() => expect(screen.getByRole("heading", { name: "Not signed in" })).toBeInTheDocument());
    expect(screen.queryByText("Ada")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: /google/i })).toHaveAttribute("href", "/auth/google/login");
  });
});

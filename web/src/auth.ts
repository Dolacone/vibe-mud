export type Location = {
  id: string;
  display_name: string;
};

export type Route = {
  origin_id: string;
  destination_id: string;
  ap_cost: number;
};

export type Item = {
  id: string;
  display_name: string;
};

export type ItemStatus = "active" | "expired";

export type InventoryItem = {
  item: Item;
  quantity: number;
  durability_status: ItemStatus;
  durability_remaining_seconds: number | null;
  retention_remaining_seconds: number | null;
};

export type GroundItem = {
  item: Item;
  quantity: number;
  durability_status: ItemStatus;
  durability_remaining_seconds: number | null;
  retention_remaining_seconds: number | null;
};

export type GroundResource = {
  resource: Item;
  quantity: number;
};

export type GatheringOption = {
  item: Item;
  quantity: number;
  ap_cost: number;
};

export type ConversionOption = {
  item: Item;
  resource: Item;
  input_quantity: number;
  resource_yield: number;
  ap_cost: number;
};

export type ConversionMethod = {
  id: string;
  display_name: string;
  ap_cost: number;
  input: Item;
  max_input_quantity: number;
  output_resource: Item;
  resource_quantity_per_input: number;
  essence_item: Item | null;
  essence_chance_bps: number;
  essence_quantity: number;
  provider_extension_ids: number[];
};

export type BuildingExtension = {
  id: number;
  slot_index: number;
  definition_id: string;
  display_name: string;
  tier: number;
  required_ap: number;
  contributed_ap: number;
  status: "under_construction" | "completed";
  available_actions: string[];
};

export type ExtensionInstallationTarget = {
  building_id: number;
  slot_index: number;
};

export type BuildingExtensionDefinition = {
  id: string;
  display_name: string;
  tier: number;
  package_item: Item;
  required_ap: number;
  installation_targets: ExtensionInstallationTarget[];
};

export type Resource = {
  resource: Item;
  quantity: number;
};

export type CraftingResourceInput = {
  resource: Item;
  quantity: number;
};

export type CraftingItemInput = {
  item: Item;
  quantity: number;
};

export type CraftingRecipe = {
  id: string;
  display_name: string;
  base_ap_cost: number;
  resource_inputs: CraftingResourceInput[];
  item_inputs: CraftingItemInput[];
  output: Item;
  output_quantity: number;
};

export type BuildingResourceInput = {
  resource: Item;
  quantity: number;
};

export type BuildingItemInput = {
  item: Item;
  quantity: number;
};

export type BuildingRecipe = {
  id: string;
  display_name: string;
  building_level: number;
  required_ap: number;
  extension_slot_count: number;
  resource_inputs: BuildingResourceInput[];
  item_inputs: BuildingItemInput[];
};

export type Building = {
  id: number;
  owner: { id: number; display_name: string };
  recipe: { id: string; display_name: string };
  building_level: number;
  required_ap: number;
  contributed_ap: number;
  status: "under_construction" | "completed";
  extension_slot_count: number;
  max_durability_seconds: number;
  durability_status: "active" | "disabled" | null;
  durability_remaining_seconds: number | null;
  extensions?: BuildingExtension[];
  available_actions: string[];
};

export type PlayerState = {
  available_actions: string[];
  location: Location;
  routes: Route[];
  ap: number;
  carried_weight: number;
  movement_weight_threshold: number;
  inventory: InventoryItem[];
  ground_items: GroundItem[];
  ground_resources: GroundResource[];
  gathering_option: GatheringOption | null;
  conversion_option: ConversionOption | null;
  conversion_methods?: ConversionMethod[];
  building_extension_definitions?: BuildingExtensionDefinition[];
  resources: Resource[];
  crafting_recipes?: CraftingRecipe[];
  building_recipes?: BuildingRecipe[];
  buildings?: Building[];
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
  | ({ status: "success" } & PlayerState)
  | ({ status: "insufficient"; error: string } & PlayerState)
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

export type MoveResult =
  | ({ status: "success" } & PlayerState)
  | ({ status: "insufficient"; error: string } & PlayerState)
  | ({ status: "invalid"; error: string; state?: PlayerState })
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

export type GatherResult =
  | ({ status: "success" } & PlayerState)
  | ({ status: "insufficient"; error: string } & PlayerState)
  | ({ status: "invalid"; error: string; state?: PlayerState })
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

export type ConvertResult =
  | ({ status: "success"; method_id?: string; quantity?: number; resource_quantity?: number; essence_quantity?: number } & PlayerState)
  | ({ status: "insufficient"; error: string } & PlayerState)
  | ({ status: "invalid"; error: string; state?: PlayerState })
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

export type CraftResult =
  | ({ status: "success" } & PlayerState)
  | ({ status: "insufficient"; error: string } & PlayerState)
  | ({ status: "invalid"; error: string; state?: PlayerState })
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

export type BuildResult =
  | ({ status: "success" } & PlayerState)
  | ({ status: "insufficient"; error: string } & PlayerState)
  | ({ status: "invalid"; error: string } & PlayerState)
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

export type ContributeConstructionResult = BuildResult;

export type RepairBuildingRequest = {
  building_id: number;
};

export type RepairResult =
  | ({ status: "success" } & PlayerState)
  | ({ status: "conflict"; error: string } & PlayerState)
  | { status: "invalid"; error: string; state?: PlayerState }
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

export type ExtensionRequest = { building_id: number; slot_index: number; definition_id: string };
export type ExtensionIDRequest = { extension_id: number };

export type TransferAssetType = "item" | "resource";

export type ItemTransferRequest = {
  asset_type: "item";
  asset_id: string;
  quantity: number;
  item_status: ItemStatus;
};

export type ResourceTransferRequest = {
  asset_type: "resource";
  asset_id: string;
  quantity: number;
};

export type TransferRequest = ItemTransferRequest | ResourceTransferRequest;
export type DropRequest = TransferRequest;
export type PickupRequest = TransferRequest;

export type TransferResult =
  | ({ status: "success" } & PlayerState)
  | { status: "invalid"; error: string; state?: PlayerState }
  | ({ status: "conflict"; error: string } & PlayerState)
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

function isItem(value: unknown): value is Item {
  if (typeof value !== "object" || value === null) return false;
  const item = value as Record<string, unknown>;
  return isString(item.id) && isString(item.display_name);
}

function isQuantity(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value > 0;
}

function isItemStatus(value: unknown): value is ItemStatus {
  return value === "active" || value === "expired";
}

function isItemDurability(value: Record<string, unknown>): value is Record<string, unknown> & {
  durability_status: ItemStatus;
  durability_remaining_seconds: number | null;
  retention_remaining_seconds: number | null;
} {
  if (!isItemStatus(value.durability_status)) return false;
  if (value.durability_status === "active") {
    return isPositiveInteger(value.durability_remaining_seconds) && value.retention_remaining_seconds === null;
  }
  return value.durability_remaining_seconds === null && isPositiveInteger(value.retention_remaining_seconds);
}

function isResource(value: unknown): value is Resource {
  if (typeof value !== "object" || value === null) return false;
  const resource = value as Record<string, unknown>;
  return isItem(resource.resource) && typeof resource.quantity === "number" && Number.isInteger(resource.quantity) && resource.quantity >= 0;
}

function isCraftingResourceInput(value: unknown): value is CraftingResourceInput {
  if (typeof value !== "object" || value === null) return false;
  const input = value as Record<string, unknown>;
  return isItem(input.resource) && isQuantity(input.quantity);
}

function isCraftingItemInput(value: unknown): value is CraftingItemInput {
  if (typeof value !== "object" || value === null) return false;
  const input = value as Record<string, unknown>;
  return isItem(input.item) && isQuantity(input.quantity);
}

function isCraftingRecipe(value: unknown): value is CraftingRecipe {
  if (typeof value !== "object" || value === null) return false;
  const recipe = value as Record<string, unknown>;
  return (
    isString(recipe.id) &&
    isString(recipe.display_name) &&
    typeof recipe.base_ap_cost === "number" &&
    Number.isInteger(recipe.base_ap_cost) &&
    recipe.base_ap_cost > 0 &&
    Array.isArray(recipe.resource_inputs) &&
    recipe.resource_inputs.length > 0 &&
    recipe.resource_inputs.every(isCraftingResourceInput) &&
    Array.isArray(recipe.item_inputs) &&
    recipe.item_inputs.every(isCraftingItemInput) &&
    isItem(recipe.output) &&
    isQuantity(recipe.output_quantity)
  );
}

function isCraftingRecipes(value: unknown): value is CraftingRecipe[] {
  if (!Array.isArray(value) || !value.every(isCraftingRecipe)) return false;
  const ids = value.map((recipe) => recipe.id);
  return new Set(ids).size === ids.length;
}

function isPositiveInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value > 0;
}

function isNonNegativeInteger(value: unknown): value is number {
  return typeof value === "number" && Number.isInteger(value) && value >= 0;
}

function isBuildingResourceInput(value: unknown): value is BuildingResourceInput {
  if (typeof value !== "object" || value === null) return false;
  const input = value as Record<string, unknown>;
  return isItem(input.resource) && isQuantity(input.quantity);
}

function isBuildingItemInput(value: unknown): value is BuildingItemInput {
  if (typeof value !== "object" || value === null) return false;
  const input = value as Record<string, unknown>;
  return isItem(input.item) && isQuantity(input.quantity);
}

function isBuildingRecipe(value: unknown): value is BuildingRecipe {
  if (typeof value !== "object" || value === null) return false;
  const recipe = value as Record<string, unknown>;
  return (
    isString(recipe.id) &&
    isString(recipe.display_name) &&
    isPositiveInteger(recipe.building_level) &&
    isPositiveInteger(recipe.required_ap) &&
    typeof recipe.extension_slot_count === "number" &&
    Number.isInteger(recipe.extension_slot_count) &&
    recipe.extension_slot_count >= 0 &&
    Array.isArray(recipe.resource_inputs) &&
    recipe.resource_inputs.every(isBuildingResourceInput) &&
    Array.isArray(recipe.item_inputs) &&
    recipe.item_inputs.every(isBuildingItemInput) &&
    (recipe.resource_inputs.length > 0 || recipe.item_inputs.length > 0)
  );
}

function isBuildingRecipes(value: unknown): value is BuildingRecipe[] {
  if (!Array.isArray(value) || !value.every(isBuildingRecipe)) return false;
  const ids = value.map((recipe) => recipe.id);
  return new Set(ids).size === ids.length;
}

function isBuilding(value: unknown): value is Building {
  if (typeof value !== "object" || value === null) return false;
  const building = value as Record<string, unknown>;
  const owner = building.owner;
  const recipe = building.recipe;
  if (
    isPositiveInteger(building.id) &&
    typeof owner === "object" && owner !== null &&
    isPositiveInteger((owner as Record<string, unknown>).id) &&
    isString((owner as Record<string, unknown>).display_name) &&
    typeof recipe === "object" && recipe !== null &&
    isString((recipe as Record<string, unknown>).id) &&
    isString((recipe as Record<string, unknown>).display_name) &&
    isPositiveInteger(building.building_level) &&
    isPositiveInteger(building.required_ap) &&
    typeof building.contributed_ap === "number" &&
    Number.isInteger(building.contributed_ap) &&
    building.contributed_ap >= 0 &&
    building.contributed_ap <= building.required_ap &&
    (building.status === "under_construction" || building.status === "completed") &&
    typeof building.extension_slot_count === "number" &&
    Number.isInteger(building.extension_slot_count) &&
    building.extension_slot_count >= 0 &&
    isPositiveInteger(building.max_durability_seconds) &&
    isAvailableActions(building.available_actions) &&
    (building.extensions === undefined || (Array.isArray(building.extensions) && building.extensions.every(isBuildingExtension)))
  ) {
    if (building.status === "under_construction") {
      return building.durability_status === null && building.durability_remaining_seconds === null;
    }
    if (building.durability_status === "active") {
      return (
        typeof building.durability_remaining_seconds === "number" &&
        Number.isInteger(building.durability_remaining_seconds) &&
        building.durability_remaining_seconds > 0 &&
        building.durability_remaining_seconds <= building.max_durability_seconds
      );
    }
    return building.durability_status === "disabled" && building.durability_remaining_seconds === 0;
  }
  return false;
}

function isBuildings(value: unknown): value is Building[] {
  if (!Array.isArray(value) || !value.every(isBuilding)) return false;
  const ids = value.map((building) => building.id);
  return new Set(ids).size === ids.length;
}

const resourceIDs = ["food", "wood", "stone", "metal", "fiber", "hide", "medicinal", "arcane"];

function isResources(value: unknown): value is Resource[] {
  if (!Array.isArray(value) || value.length !== resourceIDs.length || !value.every(isResource)) return false;
  const ids = value.map((resource) => resource.resource.id);
  return new Set(ids).size === resourceIDs.length && resourceIDs.every((id) => ids.includes(id));
}

function isInventoryItem(value: unknown): value is InventoryItem {
  if (typeof value !== "object" || value === null) return false;
  const item = value as Record<string, unknown>;
  return isItem(item.item) && isQuantity(item.quantity) && isItemDurability(item);
}

function isGroundItem(value: unknown): value is GroundItem {
  if (typeof value !== "object" || value === null) return false;
  const groundItem = value as Record<string, unknown>;
  return isItem(groundItem.item) && isQuantity(groundItem.quantity) && isItemDurability(groundItem);
}

function isGroundResource(value: unknown): value is GroundResource {
  if (typeof value !== "object" || value === null) return false;
  const groundResource = value as Record<string, unknown>;
  return isItem(groundResource.resource) && isQuantity(groundResource.quantity);
}

function isGroundItems(value: unknown): value is GroundItem[] {
  if (!Array.isArray(value) || !value.every(isGroundItem)) return false;
  const statusesByID = new Map<string, Set<ItemStatus>>();
  for (const groundItem of value) {
    const statuses = statusesByID.get(groundItem.item.id) ?? new Set<ItemStatus>();
    if (statuses.has(groundItem.durability_status)) return false;
    statuses.add(groundItem.durability_status);
    statusesByID.set(groundItem.item.id, statuses);
  }
  return true;
}

function isGroundResources(value: unknown): value is GroundResource[] {
  if (!Array.isArray(value) || !value.every(isGroundResource)) return false;
  const ids = value.map((groundResource) => groundResource.resource.id);
  return new Set(ids).size === ids.length;
}

function isGatheringOption(value: unknown): value is GatheringOption {
  if (typeof value !== "object" || value === null) return false;
  const option = value as Record<string, unknown>;
  return (
    isItem(option.item) &&
    isQuantity(option.quantity) &&
    typeof option.ap_cost === "number" &&
    Number.isInteger(option.ap_cost) &&
    option.ap_cost > 0
  );
}

function isConversionOption(value: unknown): value is ConversionOption {
  if (typeof value !== "object" || value === null) return false;
  const option = value as Record<string, unknown>;
  return (
    isItem(option.item) &&
    isItem(option.resource) &&
    isQuantity(option.input_quantity) &&
    isQuantity(option.resource_yield) &&
    typeof option.ap_cost === "number" &&
    Number.isInteger(option.ap_cost) &&
    option.ap_cost > 0
  );
}

function isConversionMethod(value: unknown): value is ConversionMethod {
  if (typeof value !== "object" || value === null) return false;
  const method = value as Record<string, unknown>;
  return isString(method.id) && isString(method.display_name) && isPositiveInteger(method.ap_cost) && isItem(method.input) && isPositiveInteger(method.max_input_quantity) && isItem(method.output_resource) && isPositiveInteger(method.resource_quantity_per_input) && (method.essence_item === null || isItem(method.essence_item)) && isNonNegativeInteger(method.essence_chance_bps) && method.essence_chance_bps <= 10000 && isNonNegativeInteger(method.essence_quantity) && Array.isArray(method.provider_extension_ids) && method.provider_extension_ids.every(isPositiveInteger);
}

function isConversionMethods(value: unknown): value is ConversionMethod[] {
  return Array.isArray(value) && value.every(isConversionMethod) && new Set(value.map((method) => method.id)).size === value.length;
}

function isBuildingExtension(value: unknown): value is BuildingExtension {
  if (typeof value !== "object" || value === null) return false;
  const extension = value as Record<string, unknown>;
  return isPositiveInteger(extension.id) && isNonNegativeInteger(extension.slot_index) && isString(extension.definition_id) && isString(extension.display_name) && isPositiveInteger(extension.tier) && isPositiveInteger(extension.required_ap) && isNonNegativeInteger(extension.contributed_ap) && extension.contributed_ap <= extension.required_ap && (extension.status === "under_construction" || extension.status === "completed") && isAvailableActions(extension.available_actions);
}

function isBuildingExtensionDefinition(value: unknown): value is BuildingExtensionDefinition {
  if (typeof value !== "object" || value === null) return false;
  const definition = value as Record<string, unknown>;
  return isString(definition.id) && isString(definition.display_name) && isPositiveInteger(definition.tier) && isItem(definition.package_item) && isPositiveInteger(definition.required_ap) && Array.isArray(definition.installation_targets) && definition.installation_targets.every(isExtensionInstallationTarget);
}

function isExtensionInstallationTarget(value: unknown): value is ExtensionInstallationTarget {
  if (typeof value !== "object" || value === null) return false;
  const target = value as Record<string, unknown>;
  return isPositiveInteger(target.building_id) && isNonNegativeInteger(target.slot_index);
}

const gameplayActions = new Set(["rest", "move", "gather", "convert", "craft", "build", "contribute-construction", "repair-building", "install-extension", "contribute-extension-construction", "remove-extension"]);

function isAvailableActions(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((action) => typeof action === "string" && gameplayActions.has(action)) && new Set(value).size === value.length;
}

function isPlayerState(value: unknown): value is PlayerState {
  if (typeof value !== "object" || value === null) return false;
  const state = value as Record<string, unknown>;
  const location = state.location;
  const routes = state.routes;
  return (
    isAvailableActions(state.available_actions) &&
    isLocation(location) &&
    Array.isArray(routes) &&
    routes.every(isRoute) &&
    routes.every((route) => route.origin_id === location.id) &&
    isAP(state.ap) &&
    isNonNegativeInteger(state.carried_weight) &&
    isPositiveInteger(state.movement_weight_threshold) &&
    Array.isArray(state.inventory) &&
    state.inventory.every(isInventoryItem) &&
    isGroundItems(state.ground_items) &&
    isGroundResources(state.ground_resources) &&
    (state.gathering_option === null || isGatheringOption(state.gathering_option)) &&
    (state.conversion_option === null || isConversionOption(state.conversion_option)) &&
    (state.conversion_methods === undefined || isConversionMethods(state.conversion_methods)) &&
    (state.building_extension_definitions === undefined || (Array.isArray(state.building_extension_definitions) && state.building_extension_definitions.every(isBuildingExtensionDefinition))) &&
    isResources(state.resources) &&
    isCraftingRecipes(state.crafting_recipes) &&
    isBuildingRecipes(state.building_recipes) &&
    isBuildings(state.buildings)
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

function isRestResponse(value: unknown): value is PlayerState {
  return isPlayerState(value);
}

function isRestConflict(value: unknown): value is { error: string } & PlayerState {
  if (typeof value !== "object" || value === null) return false;
  const body = value as Record<string, unknown>;
  return typeof body.error === "string" && isPlayerState(value);
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

function isConvertError(value: unknown): value is { error: string } {
  return typeof value === "object" && value !== null && typeof (value as Record<string, unknown>).error === "string";
}

function isConvertConflict(value: unknown): value is { error: string } & PlayerState {
  return isConvertError(value) && isPlayerState(value);
}

function isCraftError(value: unknown): value is { error: string } {
  return typeof value === "object" && value !== null && typeof (value as Record<string, unknown>).error === "string";
}

function isCraftConflict(value: unknown): value is { error: string } & PlayerState {
  return isCraftError(value) && isPlayerState(value);
}

function isRepairError(value: unknown): value is { error: string } {
  return typeof value === "object" && value !== null && typeof (value as Record<string, unknown>).error === "string";
}

function isRepairConflict(value: unknown): value is { error: string } & PlayerState {
  return isRepairError(value) && isPlayerState(value);
}

function isTransferAssetType(value: unknown): value is TransferAssetType {
  return value === "item" || value === "resource";
}

function isTransferRequest(value: unknown): value is TransferRequest {
  if (typeof value !== "object" || value === null) return false;
  const request = value as Record<string, unknown>;
  if (!isTransferAssetType(request.asset_type) || !isString(request.asset_id) || !isQuantity(request.quantity)) return false;
  if (request.asset_type === "resource") return !Object.prototype.hasOwnProperty.call(request, "item_status");
  return isItemStatus(request.item_status);
}

function isTransferError(value: unknown): value is { error: string } {
  return typeof value === "object" && value !== null && typeof (value as Record<string, unknown>).error === "string";
}

function isTransferStateResponse(value: unknown): value is PlayerState {
  return isPlayerState(value);
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
        available_actions: body.available_actions,
        ap: body.ap,
        carried_weight: body.carried_weight,
        movement_weight_threshold: body.movement_weight_threshold,
        location: body.location,
        routes: body.routes,
        inventory: body.inventory,
        ground_items: body.ground_items,
        ground_resources: body.ground_resources,
        gathering_option: body.gathering_option,
        conversion_option: body.conversion_option,
        conversion_methods: body.conversion_methods,
        building_extension_definitions: body.building_extension_definitions,
        resources: body.resources,
        crafting_recipes: body.crafting_recipes,
        building_recipes: body.building_recipes,
        buildings: body.buildings,
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
      return { ...body, status: "insufficient", error: body.error };
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
    return { ...body, status: "success" };
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
        ...body,
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
    return { status: "success", ...body };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error("move request failed"),
    };
  }
}

export async function gather(fetcher: typeof fetch = fetch): Promise<GatherResult> {
  try {
    const response = await fetcher("/api/actions/gather", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify({}),
    });

    if (response.status === 401) return { status: "unauthenticated" };

    if (response.status === 409) {
      const body: unknown = await response.json();
      if (!isMoveConflict(body)) {
        return { status: "error", error: new Error("gather response is invalid") };
      }
      return {
        status: "insufficient",
        ...body,
      };
    }

    if (response.status === 400) {
      const body: unknown = await response.json();
      if (!isMoveError(body)) {
        return { status: "error", error: new Error("gather response is invalid") };
      }
      if (isPlayerState(body)) {
        const { error, ...state } = body;
        return { status: "invalid", error, state };
      }
      return { status: "invalid", error: body.error };
    }

    if (response.status !== 200) {
      return {
        status: "error",
        error: new Error(`gather request failed with status ${response.status}`),
      };
    }

    const body: unknown = await response.json();
    if (!isPlayerState(body)) {
      return { status: "error", error: new Error("gather response is invalid") };
    }
    return { status: "success", ...body };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error("gather request failed"),
    };
  }
}

export async function convert(methodID: string | typeof fetch, quantity?: number, providerExtensionID?: number, fetcher: typeof fetch = fetch): Promise<ConvertResult> {
  const legacy = typeof methodID === "function";
  if (legacy) { fetcher = methodID as typeof fetch; methodID = ""; }
  if (!legacy && (!isString(methodID) || !isPositiveInteger(quantity) || (providerExtensionID !== undefined && !isPositiveInteger(providerExtensionID)))) return { status: "invalid", error: "invalid convert input" };
  try {
    const response = await fetcher("/api/actions/convert", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: legacy ? "{}" : JSON.stringify({ method_id: methodID, quantity, ...(providerExtensionID === undefined ? {} : { provider_extension_id: providerExtensionID }) }),
    });

    if (response.status === 401) return { status: "unauthenticated" };

    if (response.status === 409) {
      const body: unknown = await response.json();
      if (!isConvertConflict(body)) {
        return { status: "error", error: new Error("convert response is invalid") };
      }
      return {
        status: "insufficient",
        ...body,
      };
    }

    if (response.status === 400) {
      const body: unknown = await response.json();
      if (!isConvertError(body)) {
        return { status: "error", error: new Error("convert response is invalid") };
      }
      if (isPlayerState(body)) {
        const { error, ...state } = body;
        return { status: "invalid", error, state };
      }
      return { status: "invalid", error: body.error };
    }

    if (response.status !== 200) {
      return {
        status: "error",
        error: new Error(`convert request failed with status ${response.status}`),
      };
    }

    const body: unknown = await response.json();
    if (!isPlayerState(body) || (!legacy && (typeof (body as Record<string, unknown>).method_id !== "string" || !isPositiveInteger((body as Record<string, unknown>).quantity) || !isNonNegativeInteger((body as Record<string, unknown>).resource_quantity) || ((body as Record<string, unknown>).essence_quantity !== undefined && !isNonNegativeInteger((body as Record<string, unknown>).essence_quantity))))) {
      return { status: "error", error: new Error("convert response is invalid") };
    }
    return { status: "success", ...body, ...(legacy || (body as Record<string, unknown>).essence_quantity !== undefined ? {} : { essence_quantity: 0 }) } as ConvertResult;
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error("convert request failed"),
    };
  }
}

export async function craft(recipeID: string, fetcher: typeof fetch = fetch): Promise<CraftResult> {
  if (!isString(recipeID)) {
    return { status: "invalid", error: "invalid recipe identifier" };
  }

  try {
    const response = await fetcher("/api/actions/craft", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify({ recipe_id: recipeID }),
    });

    if (response.status === 401) return { status: "unauthenticated" };

    if (response.status === 409) {
      const body: unknown = await response.json();
      if (!isCraftConflict(body)) {
        return { status: "error", error: new Error("craft response is invalid") };
      }
      return { status: "insufficient", ...body };
    }

    if (response.status === 400) {
      const body: unknown = await response.json();
      if (!isCraftError(body)) {
        return { status: "error", error: new Error("craft response is invalid") };
      }
      if (!isPlayerState(body)) {
        return { status: "error", error: new Error("craft response is invalid") };
      }
      const { error, ...state } = body;
      return { status: "invalid", error, state };
    }

    if (response.status !== 200) {
      return {
        status: "error",
        error: new Error(`craft request failed with status ${response.status}`),
      };
    }

    const body: unknown = await response.json();
    if (!isPlayerState(body)) {
      return { status: "error", error: new Error("craft response is invalid") };
    }
    return { status: "success", ...body };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error("craft request failed"),
    };
  }
}

async function buildingAction(
  path: string,
  payload: Record<string, number | string>,
  action: string,
  fetcher: typeof fetch,
): Promise<BuildResult> {
  try {
    const response = await fetcher(path, {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });

    if (response.status === 401) return { status: "unauthenticated" };

    if (response.status === 409 || response.status === 400) {
      const body: unknown = await response.json();
      if (typeof body !== "object" || body === null || typeof (body as Record<string, unknown>).error !== "string") {
        return { status: "error", error: new Error(`${action} response is invalid`) };
      }
      const { error, ...state } = body as Record<string, unknown>;
      if (!isPlayerState(state)) {
        return { status: "error", error: new Error(`${action} response is invalid`) };
      }
      return { status: response.status === 409 ? "insufficient" : "invalid", error: error as string, ...state };
    }

    if (response.status !== 200) {
      return { status: "error", error: new Error(`${action} request failed with status ${response.status}`) };
    }

    const body: unknown = await response.json();
    if (!isPlayerState(body)) {
      return { status: "error", error: new Error(`${action} response is invalid`) };
    }
    return { status: "success", ...body };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error(`${action} request failed`),
    };
  }
}

export function build(recipeID: string, fetcher: typeof fetch = fetch): Promise<BuildResult> {
  if (!isString(recipeID)) return Promise.resolve({ status: "invalid", error: "invalid recipe identifier" } as BuildResult);
  return buildingAction("/api/actions/build", { recipe_id: recipeID }, "build", fetcher);
}

export function contributeConstruction(buildingID: number, ap: number, fetcher: typeof fetch = fetch): Promise<ContributeConstructionResult> {
  if (!isPositiveInteger(buildingID)) return Promise.resolve({ status: "invalid", error: "invalid building identifier" } as BuildResult);
  if (!isPositiveInteger(ap)) return Promise.resolve({ status: "invalid", error: "invalid AP" } as BuildResult);
  return buildingAction("/api/actions/contribute-construction", { building_id: buildingID, ap }, "contribute-construction", fetcher);
}

export function installExtension(request: ExtensionRequest, fetcher: typeof fetch = fetch): Promise<BuildResult> {
  if (!isPositiveInteger(request.building_id) || !isNonNegativeInteger(request.slot_index) || !isString(request.definition_id)) return Promise.resolve({ status: "invalid", error: "invalid extension input" } as BuildResult);
  return buildingAction("/api/actions/install-extension", request, "install-extension", fetcher);
}

export function contributeExtensionConstruction(extensionID: number, ap: number, fetcher: typeof fetch = fetch): Promise<BuildResult> {
  if (!isPositiveInteger(extensionID) || !isPositiveInteger(ap)) return Promise.resolve({ status: "invalid", error: "invalid extension input" } as BuildResult);
  return buildingAction("/api/actions/contribute-extension-construction", { extension_id: extensionID, ap }, "contribute-extension-construction", fetcher);
}

export function removeExtension(extensionID: number, fetcher: typeof fetch = fetch): Promise<BuildResult> {
  if (!isPositiveInteger(extensionID)) return Promise.resolve({ status: "invalid", error: "invalid extension identifier" } as BuildResult);
  return buildingAction("/api/actions/remove-extension", { extension_id: extensionID }, "remove-extension", fetcher);
}

export async function repairBuilding(buildingID: number, fetcher: typeof fetch = fetch): Promise<RepairResult> {
  if (!isPositiveInteger(buildingID)) {
    return { status: "invalid", error: "invalid building identifier" };
  }

  const request: RepairBuildingRequest = { building_id: buildingID };
  try {
    const response = await fetcher("/api/actions/repair-building", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify(request),
    });

    if (response.status === 401) return { status: "unauthenticated" };

    if (response.status === 409) {
      const body: unknown = await response.json();
      if (!isRepairConflict(body)) {
        return { status: "error", error: new Error("repair-building response is invalid") };
      }
      return { status: "conflict", ...body };
    }

    if (response.status === 400) {
      const body: unknown = await response.json();
      if (!isRepairConflict(body)) {
        return { status: "error", error: new Error("repair-building response is invalid") };
      }
      const { error, ...state } = body;
      return { status: "invalid", error, state };
    }

    if (response.status !== 200) {
      return {
        status: "error",
        error: new Error(`repair-building request failed with status ${response.status}`),
      };
    }

    const body: unknown = await response.json();
    if (!isPlayerState(body)) {
      return { status: "error", error: new Error("repair-building response is invalid") };
    }
    return { status: "success", ...body };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error("repair-building request failed"),
    };
  }
}

async function transfer(
  operation: "drop" | "pickup",
  request: TransferRequest,
  fetcher: typeof fetch,
): Promise<TransferResult> {
  if (!isTransferRequest(request)) {
    return { status: "invalid", error: "invalid transfer input" };
  }

  try {
    const response = await fetcher(`/api/transfers/${operation}`, {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: JSON.stringify({
        asset_type: request.asset_type,
        asset_id: request.asset_id,
        quantity: request.quantity,
        ...(request.asset_type === "item" ? { item_status: request.item_status } : {}),
      }),
    });

    if (response.status === 401) return { status: "unauthenticated" };

    if (response.status === 400 || response.status === 409) {
      const body: unknown = await response.json();
      if (!isTransferError(body) || !isTransferStateResponse(body)) {
        return { status: "error", error: new Error(`${operation} response is invalid`) };
      }
      const { error, ...state } = body;
      if (response.status === 400) {
        return { status: "invalid", error, state };
      }
      return {
        status: "conflict",
        error,
        ...state,
      };
    }

    if (response.status !== 200) {
      return {
        status: "error",
        error: new Error(`${operation} request failed with status ${response.status}`),
      };
    }

    const body: unknown = await response.json();
    if (!isTransferStateResponse(body)) {
      return { status: "error", error: new Error(`${operation} response is invalid`) };
    }
    return { status: "success", ...body };
  } catch (error) {
    return {
      status: "error",
      error: error instanceof Error ? error : new Error(`${operation} request failed`),
    };
  }
}

export function drop(request: DropRequest, fetcher: typeof fetch = fetch): Promise<TransferResult> {
  return transfer("drop", request, fetcher);
}

export function pickup(request: PickupRequest, fetcher: typeof fetch = fetch): Promise<TransferResult> {
  return transfer("pickup", request, fetcher);
}

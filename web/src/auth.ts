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

export type InventoryItem = {
  item: Item;
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
};

export type PlayerState = {
  location: Location;
  routes: Route[];
  ap: number;
  inventory: InventoryItem[];
  gathering_option: GatheringOption | null;
  conversion_option: ConversionOption | null;
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

export type GatherResult =
  | ({ status: "success" } & PlayerState)
  | ({ status: "insufficient"; error: string } & PlayerState)
  | ({ status: "invalid"; error: string; state?: PlayerState })
  | { status: "unauthenticated" }
  | { status: "error"; error: Error };

export type ConvertResult =
  | ({ status: "success" } & PlayerState)
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
  return (
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
    building.extension_slot_count >= 0
  );
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
  return isItem(item.item) && isQuantity(item.quantity);
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
    isAP(state.ap) &&
    Array.isArray(state.inventory) &&
    state.inventory.every(isInventoryItem) &&
    (state.gathering_option === null || isGatheringOption(state.gathering_option)) &&
    (state.conversion_option === null || isConversionOption(state.conversion_option)) &&
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
        inventory: body.inventory,
        gathering_option: body.gathering_option,
        conversion_option: body.conversion_option,
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

export async function convert(fetcher: typeof fetch = fetch): Promise<ConvertResult> {
  try {
    const response = await fetcher("/api/actions/convert", {
      method: "POST",
      credentials: "include",
      headers: { Accept: "application/json", "Content-Type": "application/json" },
      body: "{}",
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
    if (!isPlayerState(body)) {
      return { status: "error", error: new Error("convert response is invalid") };
    }
    return { status: "success", ...body };
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

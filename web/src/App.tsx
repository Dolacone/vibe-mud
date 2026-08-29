import { useEffect, useRef, useState, type ReactNode } from "react";
import { build, contributeConstruction, contributeExtensionConstruction, convert, craft, drop, gather, getCurrentUser, installExtension, move, pickup, removeExtension, repairBuilding, rest, type AuthResult, type BuildResult, type ConvertResult, type CraftResult, type CurrentUser, type GatherResult, type ItemStatus, type MoveResult, type PlayerState, type RepairResult, type RestResult, type TransferAssetType, type TransferRequest, type TransferResult } from "./auth";

type PageState = AuthResult | { status: "loading" };
type ActionState = RestResult | MoveResult | GatherResult | ConvertResult | CraftResult | BuildResult | RepairResult | TransferResult | { status: "idle" } | { status: "pending" };
type ActionKind = "rest" | "move" | "gather" | "convert" | "craft" | "build" | "contribute-construction" | "install-extension" | "contribute-extension-construction" | "remove-extension" | "repair-building" | "drop" | "pickup" | null;
type ActiveActionKind = Exclude<ActionKind, null>;

function TableScroll({ children }: { children: ReactNode }) {
  return <div className="table-scroll">{children}</div>;
}

function EmptyRow({ colSpan, children }: { colSpan: number; children: ReactNode }) {
  return <tr><td className="empty-state" colSpan={colSpan}>{children}</td></tr>;
}

function formatInputs(inputs: { resource?: { display_name: string }; item?: { display_name: string }; quantity: number }[]) {
  return inputs.length === 0 ? "None" : inputs.map((input) => `${input.resource?.display_name ?? input.item?.display_name}: ${input.quantity}`).join(", ");
}

function AuthenticatedPage({ user }: { user: CurrentUser }) {
  const [currentUser, setCurrentUser] = useState(user);
  const [action, setAction] = useState<ActionState>({ status: "idle" });
  const [actionKind, setActionKind] = useState<ActionKind>(null);
  const actionPending = useRef(false);

  const applyPlayerState = (state: PlayerState) => {
    setCurrentUser((previous) => ({ ...previous, ...state }));
  };

  const runAction = async <T extends ActionState>(kind: ActiveActionKind, request: () => Promise<T>, applyResult?: (result: T) => void) => {
    if (actionPending.current) return;
    actionPending.current = true;
    setActionKind(kind);
    setAction({ status: "pending" });
    try {
      const next = await request();
      applyResult?.(next);
      setAction(next);
    } catch (error) {
      setAction({
        status: "error",
        error: error instanceof Error ? error : new Error(`${kind} request failed`),
      });
    } finally {
      actionPending.current = false;
    }
  };

  const applyPlayerActionResult = (next: MoveResult | GatherResult | ConvertResult | CraftResult) => {
    if (next.status === "success" || next.status === "insufficient") {
      applyPlayerState(next);
    } else if (next.status === "invalid" && next.state) {
      applyPlayerState(next.state);
    }
  };

  const handleRest = () => runAction("rest", rest, (next) => {
    if (next.status === "success" || next.status === "insufficient") {
      applyPlayerState(next);
    }
  });

  const handleMove = (target: string) => runAction("move", () => move(target), applyPlayerActionResult);

  const handleGather = () => runAction("gather", gather, applyPlayerActionResult);

  const handleConvert = (methodID: string, quantity: number, providerID?: number) => runAction("convert", () => convert(methodID, quantity, providerID), applyPlayerActionResult);
  const handleLegacyConvert = () => runAction("convert", () => convert(fetch), applyPlayerActionResult);
  const handleInstall = (buildingID: number, slotIndex: number, definitionID: string) => runAction("install-extension", () => installExtension({ building_id: buildingID, slot_index: slotIndex, definition_id: definitionID }), applyBuildingResult);

  const handleCraft = (recipeID: string) => runAction("craft", () => craft(recipeID), applyPlayerActionResult);

  const applyBuildingResult = (next: BuildResult) => {
    if (next.status === "success" || next.status === "insufficient" || next.status === "invalid") {
      applyPlayerState(next);
    }
  };

  const applyBuildingAction = (kind: "build" | "contribute-construction" | "contribute-extension-construction" | "remove-extension", request: () => Promise<BuildResult>) => {
    return runAction(kind, request, applyBuildingResult);
  };

  const handleRepair = (buildingID: number) => runAction("repair-building", () => repairBuilding(buildingID), (next) => {
    if (next.status === "success" || next.status === "conflict") {
      applyPlayerState(next);
    } else if (next.status === "invalid" && next.state) {
      applyPlayerState(next.state);
    }
  });

  const applyTransferResult = (next: TransferResult) => {
    if (next.status === "success" || next.status === "conflict") {
      applyPlayerState(next);
    } else if (next.status === "invalid" && next.state) {
      applyPlayerState(next.state);
    }
  };

  const handleTransfer = (operation: "drop" | "pickup", request: TransferRequest) => {
    const transfer = operation === "drop" ? drop : pickup;
    return runAction(operation, () => transfer(request), applyTransferResult);
  };

  const actionPendingNow = action.status === "pending";
  const isOverweight = currentUser.carried_weight > currentUser.movement_weight_threshold;
  const hasAction = (name: string) => currentUser.available_actions.includes(name);

  return (
    <>
      <h2>Signed in</h2>
      <section aria-labelledby="summary-heading">
        <h2 id="summary-heading">Player summary</h2>
        <TableScroll>
          <table aria-label="Player summary">
            <tbody>
              <tr><th scope="row">ID</th><td>{currentUser.id}</td></tr>
              <tr><th scope="row">Display name</th><td>{currentUser.display_name}</td></tr>
              <tr><th scope="row">Email</th><td>{currentUser.email}</td></tr>
              <tr><th scope="row">AP</th><td>{currentUser.ap}</td></tr>
              <tr><th scope="row">Carrying weight</th><td>{currentUser.carried_weight}</td></tr>
              <tr><th scope="row">Movement weight threshold</th><td>{currentUser.movement_weight_threshold}</td></tr>
              <tr><th scope="row">Current location</th><td>{currentUser.location.display_name}</td></tr>
            </tbody>
          </table>
        </TableScroll>
      </section>
      <section aria-labelledby="actions-heading">
        <h2 id="actions-heading">Actions</h2>
        <TableScroll>
          <table aria-label="Actions">
            <thead><tr><th scope="col">Action</th><th scope="col">Cost</th><th scope="col">Controls</th></tr></thead>
            <tbody>{hasAction("rest") ? <tr><th scope="row">Rest</th><td>1 AP</td><td><button type="button" onClick={() => void handleRest()} disabled={actionPendingNow}>{actionPendingNow && actionKind === "rest" ? "Resting..." : "Rest"}</button></td></tr> : <EmptyRow colSpan={3}>No actions available.</EmptyRow>}</tbody>
          </table>
        </TableScroll>
        {action.status === "success" && actionKind === "rest" && <p role="status">Rest succeeded. AP: {currentUser.ap}</p>}
        {action.status === "insufficient" && actionKind === "rest" && <p role="alert">{action.error}</p>}
        {action.status === "unauthenticated" && <p role="alert">Your session has expired.</p>}
        {action.status === "error" && <p role="alert">{action.error.message}</p>}
      </section>
      <section aria-labelledby="resources-heading">
        <h2 id="resources-heading">Resources</h2>
        <TableScroll>
          <table aria-label="Resources">
            <thead><tr><th scope="col">Resource</th><th scope="col">Quantity</th><th scope="col">Controls</th></tr></thead>
            <tbody>{currentUser.resources.map((entry) => <tr key={entry.resource.id}><th scope="row">{entry.resource.display_name}</th><td>{entry.resource.display_name}: {entry.quantity}</td><td><TransferQuantity operation="drop" assetType="resource" assetID={entry.resource.id} displayName={entry.resource.display_name} max={entry.quantity} disabled={actionPendingNow} onSubmit={(quantity) => void handleTransfer("drop", { asset_type: "resource", asset_id: entry.resource.id, quantity })} /></td></tr>)}</tbody>
          </table>
        </TableScroll>
      </section>
      <section aria-labelledby="location-heading">
        <h2 id="location-heading">Location</h2>
        <h3>Available routes</h3>
        <p>Current location: {currentUser.location.display_name}</p>
        <TableScroll>
          <table aria-label="Available routes">
            <thead><tr><th scope="col">Destination</th><th scope="col">Cost</th><th scope="col">Controls</th></tr></thead>
            <tbody>{!hasAction("move") || currentUser.routes.length === 0 ? <EmptyRow colSpan={3}>No available routes.</EmptyRow> : currentUser.routes.map((route) => <tr key={`${route.origin_id}-${route.destination_id}`}><th scope="row">To {route.destination_id} ({route.ap_cost} AP)</th><td>{route.ap_cost} AP</td><td><button type="button" onClick={() => void handleMove(route.destination_id)} disabled={actionPendingNow}>{actionPendingNow && actionKind === "move" ? "Moving..." : `Move to ${route.destination_id}`}</button></td></tr>)}</tbody>
          </table>
        </TableScroll>
        {isOverweight && <p role="alert">Cannot move while overweight.</p>}
        {action.status === "success" && actionKind === "move" && (
          <p role="status">Move succeeded. Current location: {currentUser.location.display_name}</p>
        )}
        {action.status === "insufficient" && actionKind === "move" && (
          <p role="alert">Move failed: {action.error}</p>
        )}
        {action.status === "invalid" && actionKind === "move" && <p role="alert">Move failed: {action.error}</p>}
      </section>
      <section aria-labelledby="gather-heading">
        <h2 id="gather-heading">Gather</h2>
        <TableScroll>
          <table aria-label="Gather">
            <thead><tr><th scope="col">Yield</th><th scope="col">Cost</th><th scope="col">Controls</th></tr></thead>
            <tbody>{!hasAction("gather") || currentUser.gathering_option === null ? <EmptyRow colSpan={3}>No gathering action available.</EmptyRow> : <tr><th scope="row">Yield: {currentUser.gathering_option.quantity} {currentUser.gathering_option.item.display_name}; Cost: {currentUser.gathering_option.ap_cost} AP</th><td>{currentUser.gathering_option.ap_cost} AP</td><td><button type="button" onClick={() => void handleGather()} disabled={actionPendingNow}>{actionPendingNow && actionKind === "gather" ? "Gathering..." : "Gather"}</button></td></tr>}</tbody>
          </table>
        </TableScroll>
        {action.status === "success" && actionKind === "gather" && <p role="status">Gather succeeded.</p>}
        {action.status === "insufficient" && actionKind === "gather" && <p role="alert">Gather failed: {action.error}</p>}
        {action.status === "invalid" && actionKind === "gather" && <p role="alert">Gather failed: {action.error}</p>}
      </section>
      <section aria-labelledby="convert-heading">
        <h2 id="convert-heading">Convert</h2>
        <TableScroll>
          <table aria-label="Convert">
            <thead><tr><th scope="col">Method</th><th scope="col">Cost</th><th scope="col">Input</th><th scope="col">Output</th><th scope="col">Essence chance</th><th scope="col">Controls</th></tr></thead>
            <tbody>{!hasAction("convert") || ((currentUser.conversion_methods ?? []).length === 0 && currentUser.conversion_option === null) ? <EmptyRow colSpan={6}>No conversion action available.</EmptyRow> : (currentUser.conversion_methods ?? []).length > 0 ? (currentUser.conversion_methods ?? []).map((method) => <ConversionRow key={method.id} method={method} buildings={currentUser.buildings} disabled={actionPendingNow} onSubmit={(quantity, providerID) => void handleConvert(method.id, quantity, providerID)} />) : <LegacyConversionRow option={currentUser.conversion_option!} disabled={actionPendingNow} onSubmit={() => void handleLegacyConvert()} />}</tbody>
          </table>
        </TableScroll>
        {action.status === "success" && actionKind === "convert" && <><p role="status">Convert succeeded.</p><p role="status">Essence: {"essence_quantity" in action ? action.essence_quantity ?? 0 : 0}</p></>}
        {action.status === "insufficient" && actionKind === "convert" && <p role="alert">Convert failed: {action.error}</p>}
        {action.status === "invalid" && actionKind === "convert" && <p role="alert">Convert failed: {action.error}</p>}
      </section>
      <section aria-labelledby="craft-heading">
        <h2 id="craft-heading">Craft</h2>
        <TableScroll>
          <table aria-label="Craft">
            <thead><tr><th scope="col">Recipe</th><th scope="col">AP cost</th><th scope="col">Resource inputs</th><th scope="col">Item inputs</th><th scope="col">Output</th><th scope="col">Controls</th></tr></thead>
            <tbody>{!hasAction("craft") || (currentUser.crafting_recipes ?? []).length === 0 ? <EmptyRow colSpan={6}>No crafting recipes available.</EmptyRow> : (currentUser.crafting_recipes ?? []).map((recipe) => <tr key={recipe.id}><th scope="row">{recipe.display_name}</th><td>AP cost: {recipe.base_ap_cost}</td><td>{formatInputs(recipe.resource_inputs)}</td><td>{formatInputs(recipe.item_inputs)}</td><td>Output: {recipe.output.display_name}: {recipe.output_quantity}</td><td><button type="button" onClick={() => void handleCraft(recipe.id)} disabled={actionPendingNow}>{actionPendingNow && actionKind === "craft" ? "Crafting..." : `Craft ${recipe.display_name}`}</button></td></tr>)}</tbody>
          </table>
        </TableScroll>
        {action.status === "success" && actionKind === "craft" && <p role="status">Craft succeeded.</p>}
        {action.status === "insufficient" && actionKind === "craft" && <p role="alert">Craft failed: {action.error}</p>}
        {action.status === "invalid" && actionKind === "craft" && <p role="alert">Craft failed: {action.error}</p>}
      </section>
      <section aria-labelledby="building-heading">
        <h2 id="building-heading">Buildings</h2>
        <h3>Available recipes</h3>
        <TableScroll>
          <table aria-label="Building recipes">
            <thead><tr><th scope="col">Recipe</th><th scope="col">Required AP</th><th scope="col">Extension slots</th><th scope="col">Resource inputs</th><th scope="col">Item inputs</th><th scope="col">Controls</th></tr></thead>
            <tbody>{!hasAction("build") || (currentUser.building_recipes ?? []).length === 0 ? <EmptyRow colSpan={6}>No building recipes available.</EmptyRow> : (currentUser.building_recipes ?? []).map((recipe) => <tr key={recipe.id}><th scope="row">{recipe.display_name}</th><td>Required AP: {recipe.required_ap}</td><td>Extension slots: {recipe.extension_slot_count}</td><td>{formatInputs(recipe.resource_inputs)}</td><td>{formatInputs(recipe.item_inputs)}</td><td><button type="button" onClick={() => void applyBuildingAction("build", () => build(recipe.id))} disabled={actionPendingNow}>{actionPendingNow && actionKind === "build" ? "Building..." : `Build ${recipe.display_name}`}</button></td></tr>)}</tbody>
          </table>
        </TableScroll>
        <h3>Current location buildings</h3>
        {hasAction("install-extension") && (currentUser.building_extension_definitions ?? []).length > 0 && <TableScroll><table aria-label="Building extension definitions"><thead><tr><th scope="col">Extension</th><th scope="col">Tier</th><th scope="col">Package item</th><th scope="col">Required AP</th><th scope="col">Controls</th></tr></thead><tbody>{(currentUser.building_extension_definitions ?? []).map((definition) => <tr key={definition.id}><th scope="row">{definition.display_name} T{definition.tier}</th><td>T{definition.tier}</td><td>{definition.package_item.display_name}</td><td>{definition.required_ap}</td><td>{definition.installation_targets.map((target) => <button key={`${target.building_id}-${target.slot_index}`} type="button" onClick={() => void handleInstall(target.building_id, target.slot_index, definition.id)} disabled={actionPendingNow}>Install in building {target.building_id}, slot {target.slot_index}</button>)}</td></tr>)}</tbody></table></TableScroll>}
        <TableScroll>
          <table aria-label="Buildings">
            <thead><tr><th scope="col">Building</th><th scope="col">Owner</th><th scope="col">Status</th><th scope="col">Progress</th><th scope="col">Durability</th><th scope="col">Controls</th></tr></thead>
            <tbody>{(currentUser.buildings ?? []).length === 0 ? <EmptyRow colSpan={6}>No buildings at this location.</EmptyRow> : (currentUser.buildings ?? []).map((building) => {
              const percentage = Math.floor((building.contributed_ap / building.required_ap) * 100);
              const extensions = building.extensions ?? [];
              return <tr key={building.id}><th scope="row">{building.recipe.display_name}</th><td>Owner: {building.owner.display_name}</td><td>Status: {building.status}</td><td><span>Progress: {building.contributed_ap}/{building.required_ap} AP ({percentage}%)</span><br /><span>Empty extension slots: {Math.max(0, building.extension_slot_count - extensions.length)}</span>{extensions.map((extension) => <div key={extension.id}>{extension.display_name} T{extension.tier}: {extension.status} {extension.contributed_ap}/{extension.required_ap} AP {extension.available_actions.includes("contribute-extension-construction") && <BuildingContribution buildingID={extension.id} disabled={actionPendingNow} onSubmit={(ap) => void applyBuildingAction("contribute-extension-construction", () => contributeExtensionConstruction(extension.id, ap))} />}{extension.available_actions.includes("remove-extension") && <button type="button" onClick={() => void applyBuildingAction("remove-extension", () => removeExtension(extension.id))} disabled={actionPendingNow}>Remove extension</button>}</div>)}</td><td>{building.status === "completed" && building.durability_status !== null && building.durability_remaining_seconds !== null ? <><span>Durability status: {building.durability_status}</span><br /><span>Remaining durability: {Math.max(0, building.durability_remaining_seconds)} seconds</span></> : "-"}</td><td>{building.available_actions.includes("repair-building") && <button type="button" onClick={() => void handleRepair(building.id)} disabled={actionPendingNow}>{actionPendingNow && actionKind === "repair-building" ? "Repairing..." : `Repair building ${building.id}`}</button>}{building.available_actions.includes("contribute-construction") && <BuildingContribution buildingID={building.id} disabled={actionPendingNow} onSubmit={(ap) => void applyBuildingAction("contribute-construction", () => contributeConstruction(building.id, ap))} />}</td></tr>;
            })}</tbody>
          </table>
        </TableScroll>
        {action.status === "success" && actionKind === "build" && <p role="status">Building construction started.</p>}
        {action.status === "success" && actionKind === "contribute-construction" && <p role="status">Construction contribution succeeded.</p>}
        {(action.status === "insufficient" || action.status === "invalid") && (actionKind === "build" || actionKind === "contribute-construction") && <p role="alert">Building action failed: {action.error}</p>}
        {action.status === "success" && actionKind === "repair-building" && <p role="status">Building repair succeeded.</p>}
        {(action.status === "conflict" || action.status === "invalid") && actionKind === "repair-building" && <p role="alert">Building repair failed: {action.error}</p>}
      </section>
      <section aria-labelledby="ground-heading">
        <h2 id="ground-heading">Ground</h2>
        <h3>Ground Items</h3>
        <TableScroll>
          <table aria-label="Ground Items">
            <thead><tr><th scope="col">Item</th><th scope="col">Quantity</th><th scope="col">Status</th><th scope="col">Remaining time</th><th scope="col">Controls</th></tr></thead>
            <tbody>{currentUser.ground_items.length === 0 ? <EmptyRow colSpan={5}>Ground items are empty.</EmptyRow> : currentUser.ground_items.map((entry) => <tr key={`${entry.item.id}-${entry.durability_status}`}><th scope="row">{entry.item.display_name}</th><td>{entry.quantity}</td><td>Status: {entry.durability_status}</td><td>{entry.durability_status === "active" ? `Remaining durability: ${entry.durability_remaining_seconds ?? 0} seconds` : `Remaining retention: ${entry.retention_remaining_seconds ?? 0} seconds`}</td><td>{entry.durability_status === "active" && <TransferQuantity operation="pickup" assetType="item" assetID={entry.item.id} itemStatus={entry.durability_status} displayName={entry.item.display_name} max={entry.quantity} disabled={actionPendingNow} onSubmit={(quantity) => void handleTransfer("pickup", { asset_type: "item", asset_id: entry.item.id, quantity, item_status: entry.durability_status })} />}</td></tr>)}</tbody>
          </table>
        </TableScroll>
        <h3>Ground Resources</h3>
        <TableScroll>
          <table aria-label="Ground Resources">
            <thead><tr><th scope="col">Resource</th><th scope="col">Quantity</th><th scope="col">Controls</th></tr></thead>
            <tbody>{currentUser.ground_resources.length === 0 ? <EmptyRow colSpan={3}>Ground resources are empty.</EmptyRow> : currentUser.ground_resources.map((entry) => <tr key={entry.resource.id}><th scope="row">{entry.resource.display_name}</th><td>{entry.quantity}</td><td><TransferQuantity operation="pickup" assetType="resource" assetID={entry.resource.id} displayName={entry.resource.display_name} max={entry.quantity} disabled={actionPendingNow} onSubmit={(quantity) => void handleTransfer("pickup", { asset_type: "resource", asset_id: entry.resource.id, quantity })} /></td></tr>)}</tbody>
          </table>
        </TableScroll>
        {action.status === "success" && (actionKind === "drop" || actionKind === "pickup") && <p role="status">{actionKind === "drop" ? "Drop" : "Pickup"} succeeded.</p>}
        {(action.status === "conflict" || action.status === "invalid") && (actionKind === "drop" || actionKind === "pickup") && <p role="alert">Transfer failed: {action.error}</p>}
      </section>
      <section aria-labelledby="inventory-heading">
        <h2 id="inventory-heading">Inventory</h2>
        <TableScroll>
          <table aria-label="Inventory">
            <thead><tr><th scope="col">Item</th><th scope="col">Quantity</th><th scope="col">Status</th><th scope="col">Remaining time</th><th scope="col">Controls</th></tr></thead>
            <tbody>{currentUser.inventory.length === 0 ? <EmptyRow colSpan={5}>Inventory is empty.</EmptyRow> : currentUser.inventory.map((entry) => <tr key={`${entry.item.id}-${entry.durability_status}`}><th scope="row">{entry.item.display_name}</th><td>{entry.item.display_name}: {entry.quantity}</td><td>Status: {entry.durability_status}</td><td>{entry.durability_status === "active" ? `Remaining durability: ${entry.durability_remaining_seconds ?? 0} seconds` : `Remaining retention: ${entry.retention_remaining_seconds ?? 0} seconds`}</td><td><TransferQuantity operation="drop" assetType="item" assetID={entry.item.id} itemStatus={entry.durability_status} displayName={entry.item.display_name} max={entry.quantity} disabled={actionPendingNow} onSubmit={(quantity) => void handleTransfer("drop", { asset_type: "item", asset_id: entry.item.id, quantity, item_status: entry.durability_status })} /></td></tr>)}</tbody>
          </table>
        </TableScroll>
      </section>
    </>
  );
}

function TransferQuantity({ operation, assetType, assetID, itemStatus, displayName, max, disabled, onSubmit }: { operation: "drop" | "pickup"; assetType: TransferAssetType; assetID: string; itemStatus?: ItemStatus; displayName: string; max: number; disabled: boolean; onSubmit: (quantity: number) => void }) {
  const [quantity, setQuantity] = useState("1");
  const parsedQuantity = Number(quantity);
  const valid = Number.isInteger(parsedQuantity) && parsedQuantity > 0;
  const operationLabel = operation === "drop" ? "Drop" : "Pickup";
  const statusSuffix = itemStatus === "expired" ? " (expired)" : "";
  const unavailable = max <= 0;
  return (
    <form onSubmit={(event) => { event.preventDefault(); if (valid) onSubmit(parsedQuantity); }}>
      <label>
        {operationLabel} quantity{statusSuffix}
        <input aria-label={`${operationLabel} quantity for ${displayName}${statusSuffix}`} type="number" min="1" step="1" max={max} value={quantity} onChange={(event) => setQuantity(event.target.value)} disabled={disabled || unavailable} />
      </label>
      <button type="submit" aria-label={`${operationLabel} ${displayName}${statusSuffix}`} disabled={disabled || unavailable || !valid}>{disabled ? (operation === "drop" ? "Dropping..." : "Picking up...") : operationLabel}</button>
    </form>
  );
}

function ConversionRow({ method, buildings, disabled, onSubmit }: { method: import("./auth").ConversionMethod; buildings: CurrentUser["buildings"]; disabled: boolean; onSubmit: (quantity: number, providerID?: number) => void }) {
  const [quantity, setQuantity] = useState("1");
  const [provider, setProvider] = useState("");
  const parsed = Number(quantity);
  const valid = Number.isInteger(parsed) && parsed > 0 && parsed <= method.max_input_quantity;
  const providerRequired = method.provider_extension_ids.length > 0;
  const providers = (buildings ?? []).flatMap((building) => (building.extensions ?? []).filter((extension) => method.provider_extension_ids.includes(extension.id)));
  return <tr><th scope="row">{method.display_name}</th><td>{method.ap_cost} AP</td><td>{method.input.display_name}, max {method.max_input_quantity}</td><td>{method.resource_quantity_per_input} {method.output_resource.display_name}</td><td>{method.essence_item?.display_name ?? "None"}: {method.essence_chance_bps / 100}%</td><td><input aria-label={`Quantity for ${method.display_name}`} type="number" min="1" max={method.max_input_quantity} step="1" value={quantity} onChange={(event) => setQuantity(event.target.value)} disabled={disabled} />{providerRequired && <select aria-label={`Provider for ${method.display_name}`} value={provider} onChange={(event) => setProvider(event.target.value)} disabled={disabled}><option value="">Select provider</option>{providers.map((extension) => <option key={extension.id} value={extension.id}>{extension.display_name} T{extension.tier}</option>)}</select>}<button type="button" onClick={() => valid && (!providerRequired || provider !== "") && onSubmit(parsed, provider ? Number(provider) : undefined)} disabled={disabled || !valid || (providerRequired && provider === "")}>{disabled ? "Converting..." : "Convert"}</button></td></tr>;
}

function LegacyConversionRow({ option, disabled, onSubmit }: { option: NonNullable<CurrentUser["conversion_option"]>; disabled: boolean; onSubmit: () => void }) {
  return <tr><th scope="row">{option.item.display_name} to {option.resource.display_name}</th><td>{option.ap_cost} AP</td><td>{option.input_quantity} {option.item.display_name}</td><td>{option.resource_yield} {option.resource.display_name}</td><td>None</td><td><button type="button" onClick={onSubmit} disabled={disabled}>{disabled ? "Converting..." : "Convert"}</button></td></tr>;
}

function BuildingContribution({ buildingID, disabled, onSubmit }: { buildingID: number; disabled: boolean; onSubmit: (ap: number) => void }) {
  const [ap, setAP] = useState("");
  const parsedAP = Number(ap);
  const valid = Number.isInteger(parsedAP) && parsedAP > 0;
  return (
    <form onSubmit={(event) => { event.preventDefault(); if (valid) onSubmit(parsedAP); }}>
      <label>
        Contribution AP
        <input aria-label={`Contribution AP for building ${buildingID}`} type="number" min="1" step="1" value={ap} onChange={(event) => setAP(event.target.value)} disabled={disabled} />
      </label>
      <button type="submit" aria-label={`Contribute AP to building ${buildingID}`} disabled={disabled || !valid}>{disabled ? "Contributing..." : "Contribute AP"}</button>
    </form>
  );
}

export default function App() {
  const [result, setResult] = useState<PageState>({ status: "loading" });

  useEffect(() => {
    let active = true;
    void getCurrentUser().then((next) => {
      if (active) setResult(next);
    });
    return () => {
      active = false;
    };
  }, []);

  if (result.status === "loading") {
    return <main><h1>Vibe MUD</h1><p role="status">Loading...</p></main>;
  }

  if (result.status === "authenticated") {
    return (
      <main>
        <h1>Vibe MUD</h1>
        <AuthenticatedPage user={result.user} />
      </main>
    );
  }

  if (result.status === "unauthenticated") {
    return (
      <main>
        <h1>Vibe MUD</h1>
        <h2>Not signed in</h2>
        <p>Sign in with Google to continue.</p>
        <a className="login" href="/auth/google/login">Sign in with Google</a>
      </main>
    );
  }

  return (
    <main>
      <h1>Vibe MUD</h1>
      <h2>Unable to check sign-in</h2>
      <p role="alert">{result.error.message}</p>
      <a className="login" href="/auth/google/login">Try Google sign-in</a>
    </main>
  );
}

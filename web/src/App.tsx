import { useEffect, useRef, useState } from "react";
import { build, contributeConstruction, convert, craft, gather, getCurrentUser, move, repairBuilding, rest, type AuthResult, type BuildResult, type ConvertResult, type CraftResult, type CurrentUser, type GatherResult, type MoveResult, type PlayerState, type RepairResult, type RestResult } from "./auth";

type PageState = AuthResult | { status: "loading" };
type ActionState = RestResult | MoveResult | GatherResult | ConvertResult | CraftResult | BuildResult | RepairResult | { status: "idle" } | { status: "pending" };
type ActionKind = "rest" | "move" | "gather" | "convert" | "craft" | "build" | "contribute-construction" | "repair-building" | null;

function Identity({ user }: { user: CurrentUser }) {
  return (
    <dl className="identity">
      <div>
        <dt>ID</dt>
        <dd>{user.id}</dd>
      </div>
      <div>
        <dt>Display name</dt>
        <dd>{user.display_name}</dd>
      </div>
      <div>
        <dt>Email</dt>
        <dd>{user.email}</dd>
      </div>
    </dl>
  );
}

function AuthenticatedPage({ user }: { user: CurrentUser }) {
  const [currentUser, setCurrentUser] = useState(user);
  const [action, setAction] = useState<ActionState>({ status: "idle" });
  const [actionKind, setActionKind] = useState<ActionKind>(null);
  const actionPending = useRef(false);

  const applyPlayerState = (state: PlayerState) => {
    setCurrentUser((previous) => ({ ...previous, ...state }));
  };

  const handleRest = async () => {
    if (actionPending.current) return;
    actionPending.current = true;
    setActionKind("rest");
    setAction({ status: "pending" });
    try {
      const next = await rest();
      if (next.status === "success" || next.status === "insufficient") {
        setCurrentUser((previous) => ({ ...previous, ap: next.ap }));
      }
      setAction(next);
    } catch (error) {
      setAction({
        status: "error",
        error: error instanceof Error ? error : new Error("rest request failed"),
      });
    } finally {
      actionPending.current = false;
    }
  };

  const handleMove = async (target: string) => {
    if (actionPending.current) return;
    actionPending.current = true;
    setActionKind("move");
    setAction({ status: "pending" });
    try {
      const next = await move(target);
      if (next.status === "success" || next.status === "insufficient") {
        applyPlayerState(next);
      } else if (next.status === "invalid" && next.state) {
        applyPlayerState(next.state);
      }
      setAction(next);
    } catch (error) {
      setAction({
        status: "error",
        error: error instanceof Error ? error : new Error("move request failed"),
      });
    } finally {
      actionPending.current = false;
    }
  };

  const handleGather = async () => {
    if (actionPending.current) return;
    actionPending.current = true;
    setActionKind("gather");
    setAction({ status: "pending" });
    try {
      const next = await gather();
      if (next.status === "success" || next.status === "insufficient") {
        applyPlayerState(next);
      } else if (next.status === "invalid" && next.state) {
        applyPlayerState(next.state);
      }
      setAction(next);
    } catch (error) {
      setAction({
        status: "error",
        error: error instanceof Error ? error : new Error("gather request failed"),
      });
    } finally {
      actionPending.current = false;
    }
  };

  const handleConvert = async () => {
    if (actionPending.current) return;
    actionPending.current = true;
    setActionKind("convert");
    setAction({ status: "pending" });
    try {
      const next = await convert();
      if (next.status === "success" || next.status === "insufficient") {
        applyPlayerState(next);
      } else if (next.status === "invalid" && next.state) {
        applyPlayerState(next.state);
      }
      setAction(next);
    } catch (error) {
      setAction({
        status: "error",
        error: error instanceof Error ? error : new Error("convert request failed"),
      });
    } finally {
      actionPending.current = false;
    }
  };

  const handleCraft = async (recipeID: string) => {
    if (actionPending.current) return;
    actionPending.current = true;
    setActionKind("craft");
    setAction({ status: "pending" });
    try {
      const next = await craft(recipeID);
      if (next.status === "success" || next.status === "insufficient") {
        applyPlayerState(next);
      } else if (next.status === "invalid" && next.state) {
        applyPlayerState(next.state);
      }
      setAction(next);
    } catch (error) {
      setAction({
        status: "error",
        error: error instanceof Error ? error : new Error("craft request failed"),
      });
    } finally {
      actionPending.current = false;
    }
  };

  const applyBuildingAction = async (kind: "build" | "contribute-construction", request: () => Promise<BuildResult>) => {
    if (actionPending.current) return;
    actionPending.current = true;
    setActionKind(kind);
    setAction({ status: "pending" });
    try {
      const next = await request();
      if (next.status === "success" || next.status === "insufficient" || next.status === "invalid") {
        applyPlayerState(next);
      }
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

  const handleRepair = async (buildingID: number) => {
    if (actionPending.current) return;
    actionPending.current = true;
    setActionKind("repair-building");
    setAction({ status: "pending" });
    try {
      const next = await repairBuilding(buildingID);
      if (next.status === "success" || next.status === "conflict") {
        applyPlayerState(next);
      } else if (next.status === "invalid" && next.state) {
        applyPlayerState(next.state);
      }
      setAction(next);
    } catch (error) {
      setAction({
        status: "error",
        error: error instanceof Error ? error : new Error("repair-building request failed"),
      });
    } finally {
      actionPending.current = false;
    }
  };

  const actionPendingNow = action.status === "pending";

  return (
    <>
      <h2>Signed in</h2>
      <Identity user={currentUser} />
      <section aria-labelledby="actions-heading">
        <h2 id="actions-heading">Actions</h2>
        <p>AP: {currentUser.ap}</p>
        <h3>Resources</h3>
        <ul aria-label="Resources">
          {currentUser.resources.map((entry) => (
            <li key={entry.resource.id}>{entry.resource.display_name}: {entry.quantity}</li>
          ))}
        </ul>
        <button type="button" onClick={() => void handleRest()} disabled={actionPendingNow}>
          {actionPendingNow ? "Resting..." : "Rest"}
        </button>
        {action.status === "success" && actionKind === "rest" && <p role="status">Rest succeeded. AP: {currentUser.ap}</p>}
        {action.status === "insufficient" && actionKind === "rest" && <p role="alert">{action.error}</p>}
        {action.status === "unauthenticated" && <p role="alert">Your session has expired.</p>}
        {action.status === "error" && <p role="alert">{action.error.message}</p>}
      </section>
      <section aria-labelledby="location-heading">
        <h2 id="location-heading">Location</h2>
        <p>Current location: {currentUser.location.display_name}</p>
        <h3>Available routes</h3>
        {currentUser.routes.length === 0 ? (
          <p>No available routes.</p>
        ) : (
          <ul>
            {currentUser.routes.map((route) => (
              <li key={`${route.origin_id}-${route.destination_id}`}>
                <span>To {route.destination_id} ({route.ap_cost} AP)</span>{" "}
                <button
                  type="button"
                  onClick={() => void handleMove(route.destination_id)}
                  disabled={actionPendingNow}
                >
                  {actionPendingNow ? "Moving..." : `Move to ${route.destination_id}`}
                </button>
              </li>
            ))}
          </ul>
        )}
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
        {currentUser.gathering_option === null ? (
          <p>No gathering action available.</p>
        ) : (
          <>
            <p>
              Yield: {currentUser.gathering_option.quantity} {currentUser.gathering_option.item.display_name}; Cost: {currentUser.gathering_option.ap_cost} AP
            </p>
            <button type="button" onClick={() => void handleGather()} disabled={actionPendingNow}>
              {actionPendingNow ? "Gathering..." : "Gather"}
            </button>
          </>
        )}
        {action.status === "success" && actionKind === "gather" && <p role="status">Gather succeeded.</p>}
        {action.status === "insufficient" && actionKind === "gather" && <p role="alert">Gather failed: {action.error}</p>}
        {action.status === "invalid" && actionKind === "gather" && <p role="alert">Gather failed: {action.error}</p>}
      </section>
      <section aria-labelledby="convert-heading">
        <h2 id="convert-heading">Convert</h2>
        {currentUser.conversion_option === null ? (
          <p>No conversion action available.</p>
        ) : (
          <>
            <p>
              Input: {currentUser.conversion_option.input_quantity} {currentUser.conversion_option.item.display_name}; Yield: {currentUser.conversion_option.resource_yield} {currentUser.conversion_option.resource.display_name} Resource; Cost: {currentUser.conversion_option.ap_cost} AP
            </p>
            <button type="button" onClick={() => void handleConvert()} disabled={actionPendingNow}>
              {actionPendingNow ? "Converting..." : "Convert"}
            </button>
          </>
        )}
        {action.status === "success" && actionKind === "convert" && <p role="status">Convert succeeded.</p>}
        {action.status === "insufficient" && actionKind === "convert" && <p role="alert">Convert failed: {action.error}</p>}
        {action.status === "invalid" && actionKind === "convert" && <p role="alert">Convert failed: {action.error}</p>}
      </section>
      <section aria-labelledby="craft-heading">
        <h2 id="craft-heading">Craft</h2>
        {(currentUser.crafting_recipes ?? []).map((recipe) => (
          <article key={recipe.id} aria-labelledby={`recipe-${recipe.id}`}>
            <h3 id={`recipe-${recipe.id}`}>{recipe.display_name}</h3>
            <p>AP cost: {recipe.base_ap_cost}</p>
            <p>Resource inputs:</p>
            <ul>
              {recipe.resource_inputs.map((input) => (
                <li key={input.resource.id}>{input.resource.display_name}: {input.quantity}</li>
              ))}
            </ul>
            <p>Item inputs:</p>
            {recipe.item_inputs.length === 0 ? (
              <p>None</p>
            ) : (
              <ul>
                {recipe.item_inputs.map((input) => (
                  <li key={input.item.id}>{input.item.display_name}: {input.quantity}</li>
                ))}
              </ul>
            )}
            <p>Output: {recipe.output.display_name}: {recipe.output_quantity}</p>
            <button type="button" onClick={() => void handleCraft(recipe.id)} disabled={actionPendingNow}>
              {actionPendingNow ? "Crafting..." : `Craft ${recipe.display_name}`}
            </button>
          </article>
        ))}
        {action.status === "success" && actionKind === "craft" && <p role="status">Craft succeeded.</p>}
        {action.status === "insufficient" && actionKind === "craft" && <p role="alert">Craft failed: {action.error}</p>}
        {action.status === "invalid" && actionKind === "craft" && <p role="alert">Craft failed: {action.error}</p>}
      </section>
      <section aria-labelledby="building-heading">
        <h2 id="building-heading">Buildings</h2>
        <h3>Available recipes</h3>
        {(currentUser.building_recipes ?? []).map((recipe) => (
          <article key={recipe.id} aria-labelledby={`building-recipe-${recipe.id}`}>
            <h4 id={`building-recipe-${recipe.id}`}>{recipe.display_name}</h4>
            <p>Required AP: {recipe.required_ap}</p>
            <p>Extension slots: {recipe.extension_slot_count}</p>
            <p>Resource inputs:</p>
            {recipe.resource_inputs.length === 0 ? <p>None</p> : (
              <ul>
                {recipe.resource_inputs.map((input) => <li key={input.resource.id}>{input.resource.display_name}: {input.quantity}</li>)}
              </ul>
            )}
            <p>Item inputs:</p>
            {recipe.item_inputs.length === 0 ? <p>None</p> : (
              <ul>
                {recipe.item_inputs.map((input) => <li key={input.item.id}>{input.item.display_name}: {input.quantity}</li>)}
              </ul>
            )}
            <button type="button" onClick={() => void applyBuildingAction("build", () => build(recipe.id))} disabled={actionPendingNow}>
              {actionPendingNow && actionKind === "build" ? "Building..." : `Build ${recipe.display_name}`}
            </button>
          </article>
        ))}
        <h3>Current location buildings</h3>
        {(currentUser.buildings ?? []).length === 0 ? <p>No buildings at this location.</p> : (
          <ul aria-label="Buildings">
            {(currentUser.buildings ?? []).map((building) => {
              const percentage = Math.floor((building.contributed_ap / building.required_ap) * 100);
              const canContribute = building.status === "under_construction";
              return (
                <li key={building.id}>
                  <article aria-label={`${building.recipe.display_name} building`}>
                    <h4>{building.recipe.display_name}</h4>
                    <p>Owner: {building.owner.display_name}</p>
                    <p>Status: {building.status}</p>
                    <p>Progress: {building.contributed_ap}/{building.required_ap} AP ({percentage}%)</p>
                    <p>Empty extension slots: {building.extension_slot_count}</p>
                    {building.status === "completed" && building.durability_status !== null && building.durability_remaining_seconds !== null && (
                      <>
                        <p>Durability status: {building.durability_status}</p>
                        <p>Remaining durability: {Math.max(0, building.durability_remaining_seconds)} seconds</p>
                        <button type="button" onClick={() => void handleRepair(building.id)} disabled={actionPendingNow}>
                          {actionPendingNow && actionKind === "repair-building" ? "Repairing..." : `Repair building ${building.id}`}
                        </button>
                      </>
                    )}
                    {canContribute && (
                      <BuildingContribution buildingID={building.id} disabled={actionPendingNow} onSubmit={(ap) => void applyBuildingAction("contribute-construction", () => contributeConstruction(building.id, ap))} />
                    )}
                  </article>
                </li>
              );
            })}
          </ul>
        )}
        {action.status === "success" && actionKind === "build" && <p role="status">Building construction started.</p>}
        {action.status === "success" && actionKind === "contribute-construction" && <p role="status">Construction contribution succeeded.</p>}
        {(action.status === "insufficient" || action.status === "invalid") && (actionKind === "build" || actionKind === "contribute-construction") && <p role="alert">Building action failed: {action.error}</p>}
        {action.status === "success" && actionKind === "repair-building" && <p role="status">Building repair succeeded.</p>}
        {(action.status === "conflict" || action.status === "invalid") && actionKind === "repair-building" && <p role="alert">Building repair failed: {action.error}</p>}
      </section>
      <section aria-labelledby="inventory-heading">
        <h2 id="inventory-heading">Inventory</h2>
        {currentUser.inventory.length === 0 ? (
          <p>Inventory is empty.</p>
        ) : (
          <ul>
            {currentUser.inventory.map((entry) => (
              <li key={entry.item.id}>{entry.item.display_name}: {entry.quantity}</li>
            ))}
          </ul>
        )}
      </section>
    </>
  );
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
      <button type="submit" disabled={disabled || !valid}>{disabled ? "Contributing..." : "Contribute AP"}</button>
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

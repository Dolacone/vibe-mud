import { useEffect, useRef, useState } from "react";
import { convert, gather, getCurrentUser, move, rest, type AuthResult, type ConvertResult, type CurrentUser, type GatherResult, type MoveResult, type PlayerState, type RestResult } from "./auth";

type PageState = AuthResult | { status: "loading" };
type ActionState = RestResult | MoveResult | GatherResult | ConvertResult | { status: "idle" } | { status: "pending" };
type ActionKind = "rest" | "move" | "gather" | "convert" | null;

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

  const actionPendingNow = action.status === "pending";

  return (
    <>
      <h2>Signed in</h2>
      <Identity user={currentUser} />
      <section aria-labelledby="actions-heading">
        <h2 id="actions-heading">Actions</h2>
        <p>AP: {currentUser.ap}</p>
        <p>Resource: {currentUser.resource}</p>
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
              Input: {currentUser.conversion_option.input_quantity} {currentUser.conversion_option.item.display_name}; Yield: {currentUser.conversion_option.resource_yield} Resource; Cost: {currentUser.conversion_option.ap_cost} AP
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

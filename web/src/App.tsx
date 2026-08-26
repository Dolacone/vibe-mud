import { useEffect, useRef, useState } from "react";
import { getCurrentUser, move, rest, type AuthResult, type CurrentUser, type MoveResult, type PlayerState, type RestResult } from "./auth";

type PageState = AuthResult | { status: "loading" };
type ActionState = RestResult | MoveResult | { status: "idle" } | { status: "pending" };

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
  const actionPending = useRef(false);

  const applyPlayerState = (state: PlayerState) => {
    setCurrentUser((previous) => ({ ...previous, ...state }));
  };

  const handleRest = async () => {
    if (actionPending.current) return;
    actionPending.current = true;
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

  const actionPendingNow = action.status === "pending";

  return (
    <>
      <h2>Signed in</h2>
      <Identity user={currentUser} />
      <section aria-labelledby="actions-heading">
        <h2 id="actions-heading">Actions</h2>
        <p>AP: {currentUser.ap}</p>
        <button type="button" onClick={() => void handleRest()} disabled={actionPendingNow}>
          {actionPendingNow ? "Resting..." : "Rest"}
        </button>
        {action.status === "success" && !("location" in action) && <p role="status">Rest succeeded. AP: {currentUser.ap}</p>}
        {action.status === "insufficient" && !("location" in action) && <p role="alert">{action.error}</p>}
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
        {action.status === "success" && "location" in action && (
          <p role="status">Move succeeded. Current location: {currentUser.location.display_name}</p>
        )}
        {action.status === "insufficient" && "location" in action && (
          <p role="alert">Move failed: {action.error}</p>
        )}
        {action.status === "invalid" && <p role="alert">Move failed: {action.error}</p>}
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

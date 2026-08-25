import { useEffect, useRef, useState } from "react";
import { getCurrentUser, rest, type AuthResult, type CurrentUser, type RestResult } from "./auth";

type PageState = AuthResult | { status: "loading" };
type ActionState = RestResult | { status: "idle" } | { status: "pending" };

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
  const restPending = useRef(false);

  const handleRest = async () => {
    if (restPending.current) return;
    restPending.current = true;
    setAction({ status: "pending" });
    try {
      const next = await rest();
      if (next.status === "success") {
        setCurrentUser((previous) => ({ ...previous, ap: next.ap }));
      }
      setAction(next);
    } catch (error) {
      setAction({
        status: "error",
        error: error instanceof Error ? error : new Error("rest request failed"),
      });
    } finally {
      restPending.current = false;
    }
  };

  return (
    <>
      <h2>Signed in</h2>
      <Identity user={currentUser} />
      <section aria-labelledby="actions-heading">
        <h2 id="actions-heading">Actions</h2>
        <p>AP: {currentUser.ap}</p>
        <button type="button" onClick={() => void handleRest()} disabled={action.status === "pending"}>
          {action.status === "pending" ? "Resting..." : "Rest"}
        </button>
        {action.status === "success" && <p role="status">Rest succeeded. AP: {currentUser.ap}</p>}
        {action.status === "insufficient" && <p role="alert">{action.error}</p>}
        {action.status === "unauthenticated" && <p role="alert">Your session has expired.</p>}
        {action.status === "error" && <p role="alert">{action.error.message}</p>}
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

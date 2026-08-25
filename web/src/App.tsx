import { useEffect, useState } from "react";
import { getCurrentUser, type AuthResult, type CurrentUser } from "./auth";

type PageState = AuthResult | { status: "loading" };

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
        <h2>Signed in</h2>
        <Identity user={result.user} />
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

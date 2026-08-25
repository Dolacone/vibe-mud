import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import * as auth from "./auth";

vi.mock("./auth", async () => {
  const actual = await vi.importActual<typeof import("./auth")>("./auth");
  return { ...actual, getCurrentUser: vi.fn(), rest: vi.fn() };
});

const getCurrentUser = vi.mocked(auth.getCurrentUser);
const rest = vi.mocked(auth.rest);

describe("App", () => {
  beforeEach(() => {
    getCurrentUser.mockReset();
    rest.mockReset();
  });

  it("loads and displays only the backend-confirmed identity", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ap: 3000 } });
    render(<App />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading");
    await waitFor(() => expect(screen.getByText("Ada")).toBeInTheDocument());
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("ada@example.com")).toBeInTheDocument();
    expect(screen.getByText("AP: 3000")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled();
    expect(screen.queryByText(/role|token/i)).not.toBeInTheDocument();
  });

  it("updates the displayed AP after a successful rest", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ap: 2 } });
    rest.mockResolvedValue({ status: "success", ap: 1 });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(screen.getByText("Rest succeeded. AP: 1")).toBeInTheDocument());
    expect(screen.getByText("AP: 1")).toBeInTheDocument();
    expect(rest).toHaveBeenCalledTimes(1);
  });

  it("keeps the known AP and shows the rejection when AP is insufficient", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ap: 0 } });
    rest.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ap: 0 });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByText("AP: 0")).toBeInTheDocument();
  });

  it("updates stale AP from an insufficient rest response", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ap: 1 } });
    rest.mockResolvedValue({ status: "insufficient", error: "insufficient action points", ap: 0 });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    expect(screen.getByText("AP: 1")).toBeInTheDocument();
    button.click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("insufficient action points"));
    expect(screen.getByText("AP: 0")).toBeInTheDocument();
    expect(screen.queryByText("AP: 1")).not.toBeInTheDocument();
  });

  it("disables rest and prevents duplicate requests while one is pending", async () => {
    let resolveRest: ((value: auth.RestResult) => void) | undefined;
    rest.mockReturnValue(new Promise((resolve) => { resolveRest = resolve; }));
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ap: 2 } });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(button).toBeDisabled());
    expect(rest).toHaveBeenCalledTimes(1);
    button.click();
    expect(rest).toHaveBeenCalledTimes(1);

    resolveRest?.({ status: "success", ap: 1 });
    await waitFor(() => expect(screen.getByText("AP: 1")).toBeInTheDocument());
    expect(screen.getByRole("button", { name: "Rest" })).toBeEnabled();
  });

  it("keeps the known AP when rest fails", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ap: 2 } });
    rest.mockResolvedValue({ status: "error", error: new Error("backend unavailable") });
    render(<App />);

    const button = await screen.findByRole("button", { name: "Rest" });
    button.click();
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("backend unavailable"));
    expect(screen.getByText("AP: 2")).toBeInTheDocument();
  });

  it("offers same-origin Google login when signed out", async () => {
    getCurrentUser.mockResolvedValue({ status: "unauthenticated" });
    render(<App />);
    await waitFor(() => expect(screen.getByRole("heading", { name: "Not signed in" })).toBeInTheDocument());
    expect(screen.getByRole("link", { name: /google/i })).toHaveAttribute("href", "/auth/google/login");
  });

  it("does not retain identity after an unauthenticated response", async () => {
    getCurrentUser.mockResolvedValue({ status: "unauthenticated" });
    render(<App />);
    await waitFor(() => expect(screen.queryByText("Ada")).not.toBeInTheDocument());
    expect(screen.getByRole("heading", { name: "Not signed in" })).toBeInTheDocument();
  });

  it("shows backend failures separately", async () => {
    getCurrentUser.mockResolvedValue({ status: "error", error: new Error("backend unavailable") });
    render(<App />);
    await waitFor(() => expect(screen.getByRole("alert")).toHaveTextContent("backend unavailable"));
    expect(screen.getByRole("link", { name: /google/i })).toBeInTheDocument();
  });
});

import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";
import * as auth from "./auth";

vi.mock("./auth", async () => {
  const actual = await vi.importActual<typeof import("./auth")>("./auth");
  return { ...actual, getCurrentUser: vi.fn() };
});

const getCurrentUser = vi.mocked(auth.getCurrentUser);

describe("App", () => {
  beforeEach(() => getCurrentUser.mockReset());

  it("loads and displays only the backend-confirmed identity", async () => {
    getCurrentUser.mockResolvedValue({ status: "authenticated", user: { id: 1, display_name: "Ada", email: "ada@example.com", ap: 3000 } });
    render(<App />);
    expect(screen.getByRole("status")).toHaveTextContent("Loading");
    await waitFor(() => expect(screen.getByText("Ada")).toBeInTheDocument());
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("ada@example.com")).toBeInTheDocument();
    expect(screen.queryByText(/role|token/i)).not.toBeInTheDocument();
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

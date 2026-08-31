import { fireEvent, render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import GameShell from "./GameShell";
import "./styles.css";

const player = {
  ap: 27,
  carried_weight: 750,
  movement_weight_threshold: 1000,
  resources: [
    { resource: { id: "arcane", display_name: "Arcane" }, quantity: 4 },
    { resource: { id: "wood", display_name: "Wood" }, quantity: 2 },
    { resource: { id: "food", display_name: "Food" }, quantity: 0 },
    { resource: { id: "stone", display_name: "Stone" }, quantity: 1 },
    { resource: { id: "unknown", display_name: "Unknown" }, quantity: 9 },
  ],
};

const tabContent = {
  map: <p>地圖內容</p>,
  area: <p>地區內容</p>,
  items: <p>道具內容</p>,
  character: <p>角色內容</p>,
};

describe("GameShell", () => {
  it("names each weight boundary without relying on color", () => {
    const view = render(<GameShell player={player} tabContent={tabContent} />);
    const weight = () => screen.getByLabelText(/Weight/);

    expect(weight()).toHaveAccessibleName("Weight 750/1000");
    expect(weight()).toHaveClass("game-shell__status-item--green");
    view.rerender(<GameShell player={{ ...player, carried_weight: 751 }} tabContent={tabContent} />);
    expect(weight()).toHaveAccessibleName("Weight 751/1000");
    expect(weight()).toHaveClass("game-shell__status-item--yellow");
    view.rerender(<GameShell player={{ ...player, carried_weight: 1000 }} tabContent={tabContent} />);
    expect(weight()).toHaveAccessibleName("Weight 1000/1000");
    expect(weight()).toHaveClass("game-shell__status-item--yellow");
    view.rerender(<GameShell player={{ ...player, carried_weight: 1001 }} tabContent={tabContent} />);
    expect(weight()).toHaveAccessibleName("Weight 1001/1000");
    expect(weight()).toHaveClass("game-shell__status-item--red");
  });

  it("keeps two status rows visible and filters resources by authoritative order", () => {
    render(<GameShell player={player} tabContent={tabContent} />);

    expect(screen.getByLabelText("玩家狀態")).toBeInTheDocument();
    expect(screen.getByLabelText("目前 AP 27")).toBeInTheDocument();
    expect(screen.getByLabelText("目前 HP 尚未實作")).toHaveTextContent("HP --");
    expect(screen.getByLabelText("Weight 750/1000")).toBeInTheDocument();

    const rows = screen.getAllByLabelText(/核心狀態|Resource 持有量/);
    expect(rows).toHaveLength(2);
    expect(rows[1]).toHaveTextContent("Wood 2");
    expect(rows[1]).toHaveTextContent("Stone 1");
    expect(rows[1]).toHaveTextContent("Arcane 4");
    expect(rows[1].textContent?.indexOf("Wood 2")).toBeLessThan(rows[1].textContent?.indexOf("Stone 1") ?? 0);
    expect(rows[1].textContent?.indexOf("Stone 1")).toBeLessThan(rows[1].textContent?.indexOf("Arcane 4") ?? 0);
    expect(rows[1]).not.toHaveTextContent("Food 0");
    expect(rows[1]).not.toHaveTextContent("Unknown 9");
  });

  it("provides four labeled native navigation buttons with selected state and focus", () => {
    const onTabChange = vi.fn();
    render(<GameShell player={player} tabContent={tabContent} onTabChange={onTabChange} />);

    const navigation = screen.getByRole("navigation", { name: "主分頁" });
    const buttons = within(navigation).getAllByRole("button");
    expect(buttons.map((button) => button.getAttribute("aria-label"))).toEqual(["地圖", "地區", "道具", "角色"]);
    expect(buttons[0]).toHaveAttribute("aria-current", "page");
    expect(buttons.every((button) => button.getAttribute("type") === "button")).toBe(true);
    expect(buttons.every((button) => getComputedStyle(button).minWidth === "44px")).toBe(true);

    fireEvent.click(buttons[2]);
    expect(onTabChange).toHaveBeenCalledWith("items");
    expect(buttons[2]).toHaveAttribute("aria-current", "page");
    expect(buttons[2]).toHaveFocus();
  });

  it.each(["Enter", " "])('activates a tab with the native "%s" keyboard action', (key) => {
    render(<GameShell player={player} tabContent={tabContent} />);
    const areaButton = screen.getByRole("button", { name: "地區" });

    areaButton.focus();
    fireEvent.keyDown(areaButton, { key });

    expect(areaButton).toHaveAttribute("aria-current", "page");
    expect(areaButton).toHaveFocus();
    expect(screen.getByText("地區內容")).toBeVisible();
  });

  it("preserves supplied tab content while switching and reflects new authoritative props", () => {
    const view = render(<GameShell player={player} tabContent={tabContent} />);
    const navigation = screen.getByRole("navigation", { name: "主分頁" });

    fireEvent.click(within(navigation).getByRole("button", { name: "道具" }));
    expect(screen.getByText("地圖內容")).toBeInTheDocument();
    expect(screen.getByText("道具內容")).toBeVisible();
    expect(screen.getByLabelText("目前 AP 27")).toBeInTheDocument();

    view.rerender(<GameShell player={{ ...player, ap: 18, carried_weight: 1001 }} tabContent={tabContent} activeTab="items" />);
    expect(screen.getByLabelText("目前 AP 18")).toBeInTheDocument();
    expect(screen.getByLabelText("Weight 1001/1000")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "道具" })).toHaveAttribute("aria-current", "page");
    expect(screen.getByText("道具內容")).toBeVisible();
  });
});

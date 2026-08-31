import { useRef, useState, type KeyboardEvent, type ReactNode } from "react";
import type { Resource } from "./auth";

export const GAME_SHELL_TABS = ["map", "area", "items", "character"] as const;
export type GameShellTab = (typeof GAME_SHELL_TABS)[number];

type GameShellLabels = Record<GameShellTab, string>;

const TAB_LABELS: GameShellLabels = {
  map: "地圖",
  area: "地區",
  items: "道具",
  character: "角色",
};

const RESOURCE_ORDER = ["food", "wood", "stone", "metal", "fiber", "hide", "medicinal", "arcane"] as const;

export type GameShellPlayer = {
  ap: number;
  carried_weight: number;
  movement_weight_threshold: number;
  resources: Resource[];
};

export type GameShellProps = {
  player: GameShellPlayer;
  tabContent: Partial<Record<GameShellTab, ReactNode>>;
  activeTab?: GameShellTab;
  defaultTab?: GameShellTab;
  onTabChange?: (tab: GameShellTab) => void;
  hp?: number;
};

function orderedNonZeroResources(resources: Resource[]) {
  const byID = new Map(resources.map((entry) => [entry.resource.id, entry]));
  return RESOURCE_ORDER.flatMap((id) => {
    const entry = byID.get(id);
    return entry && entry.quantity > 0 ? [entry] : [];
  });
}

function isGameShellTab(value: string): value is GameShellTab {
  return (GAME_SHELL_TABS as readonly string[]).includes(value);
}

function weightState(current: number, maximum: number) {
  const ratio = maximum > 0 ? current / maximum : current > 0 ? Infinity : 0;
  if (ratio <= 0.75) return "safe";
  if (ratio <= 1) return "warning";
  return "overweight";
}

export function GameShell({ player, tabContent, activeTab, defaultTab = "map", onTabChange, hp }: GameShellProps) {
  const [uncontrolledTab, setUncontrolledTab] = useState<GameShellTab>(defaultTab);
  const selectedTab = activeTab ?? uncontrolledTab;
  const tabButtons = useRef<Partial<Record<GameShellTab, HTMLButtonElement>>>({});
  const visibleResources = orderedNonZeroResources(player.resources);
  const currentWeightState = weightState(player.carried_weight, player.movement_weight_threshold);
  const weightLabel = `Weight ${player.carried_weight}/${player.movement_weight_threshold} (${currentWeightState})`;

  const activateTab = (tab: GameShellTab) => {
    if (activeTab === undefined) setUncontrolledTab(tab);
    onTabChange?.(tab);
    tabButtons.current[tab]?.focus();
  };

  const handleTabKeyDown = (event: KeyboardEvent<HTMLButtonElement>, tab: GameShellTab) => {
    if (event.key !== "Enter" && event.key !== " ") return;
    event.preventDefault();
    activateTab(tab);
  };

  return (
    <div className="game-shell">
      <header className="game-shell__header" aria-label="玩家狀態">
        <div className="game-shell__status-row" aria-label="核心狀態">
          <span className="game-shell__status-item" aria-label={`目前 AP ${player.ap}`}>
            <span aria-hidden="true">AP {player.ap}</span>
          </span>
          <span className="game-shell__status-item" aria-label={hp === undefined ? "目前 HP 尚未實作" : `目前 HP ${hp}`}>
            <span aria-hidden="true">HP {hp === undefined ? "--" : hp}</span>
          </span>
          <span className={`game-shell__status-item game-shell__status-item--weight game-shell__status-item--${currentWeightState}`} aria-label={weightLabel}>
            <span aria-hidden="true">{weightLabel}</span>
          </span>
        </div>
        <div className="game-shell__status-row" aria-label="Resource 持有量">
          {visibleResources.map((entry) => (
            <span className="game-shell__status-item" key={entry.resource.id}>
              {entry.resource.display_name} {entry.quantity}
            </span>
          ))}
        </div>
      </header>

      <main className="game-shell__content" aria-label={TAB_LABELS[selectedTab]}>
        {GAME_SHELL_TABS.map((tab) => (
          <section className="game-shell__tab-panel" hidden={tab !== selectedTab} key={tab} aria-label={TAB_LABELS[tab]}>
            {tabContent[tab]}
          </section>
        ))}
      </main>

      <nav className="game-shell__navigation" aria-label="主分頁">
        {GAME_SHELL_TABS.map((tab) => (
          <button
            key={tab}
            ref={(button) => {
              if (button) tabButtons.current[tab] = button;
            }}
            type="button"
            aria-label={TAB_LABELS[tab]}
            aria-current={selectedTab === tab ? "page" : undefined}
            onClick={() => activateTab(tab)}
            onKeyDown={(event) => handleTabKeyDown(event, tab)}
          >
            {TAB_LABELS[tab]}
          </button>
        ))}
      </nav>
    </div>
  );
}

export function isGameShellTabValue(value: string): value is GameShellTab {
  return isGameShellTab(value);
}

export default GameShell;

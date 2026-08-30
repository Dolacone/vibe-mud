import { useState, type ReactNode } from "react";
import { createRoot } from "react-dom/client";
import GameShell, { type GameShellTab } from "./GameShell";
import "./styles.css";

const fixtureResources = ["food", "wood", "stone", "metal", "fiber", "hide", "medicinal", "arcane"].map((id) => ({
  resource: { id, display_name: id[0].toUpperCase() + id.slice(1) },
  quantity: 1,
}));

const fixtureParagraphs = Array.from({ length: 18 }, (_, index) => `Fixture content row ${index + 1} keeps the active panel vertically scrollable for mobile layout checks.`);

function FixturePanel({ title, children }: { title: string; children?: ReactNode }) {
  return <section><h1>{title}</h1>{children}{fixtureParagraphs.map((paragraph) => <p key={paragraph}>{paragraph}</p>)}</section>;
}

function BrowserFixture() {
  const [activeTab, setActiveTab] = useState<GameShellTab>("map");
  const tabContent = {
    map: <FixturePanel title="地圖"><p>Fixture current location: Mobile test camp.</p></FixturePanel>,
    area: <FixturePanel title="地區"><p>Fixture area controls stay inside the active content region.</p></FixturePanel>,
    items: <FixturePanel title="道具"><label>Quantity<input aria-label="Fixture quantity" data-fixture-quantity type="number" min="1" value="1" onChange={() => undefined} /></label></FixturePanel>,
    character: <FixturePanel title="角色"><p>Fixture character progression placeholders.</p></FixturePanel>,
  } satisfies Record<GameShellTab, ReactNode>;

  return <div data-fixture-root="mobile-shell"><GameShell player={{ display_name: "Long fixture player name for horizontal swipe verification", ap: 123, resources: fixtureResources }} activeTab={activeTab} onTabChange={setActiveTab} tabContent={tabContent} /></div>;
}

createRoot(document.getElementById("root")!).render(<BrowserFixture />);

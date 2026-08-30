import { describe, expect, it } from "vitest";
import fixtureHTML from "../browser-fixture.html?raw";
import indexHTML from "../index.html?raw";
import browserFixtureSource from "./browser-fixture.tsx?raw";
import styles from "./styles.css?raw";

const viewportMeta = '<meta name="viewport" content="width=device-width, initial-scale=1.0, viewport-fit=cover" />';

describe("mobile shell safe-area contract", () => {
  it("opts production and fixture documents into the full viewport", () => {
    expect(indexHTML).toContain(viewportMeta);
    expect(fixtureHTML).toContain(viewportMeta);
  });

  it("maps environment insets to fixed chrome and content clearance", () => {
    expect(styles).toContain("--shell-safe-area-top: env(safe-area-inset-top, 0px);");
    expect(styles).toContain("--shell-safe-area-bottom: env(safe-area-inset-bottom, 0px);");
    expect(styles).toContain("padding: var(--shell-safe-area-top) .75rem 0;");
    expect(styles).toContain("padding: calc(var(--shell-header-space) + var(--shell-safe-area-top)) .75rem calc(var(--shell-navigation-space) + var(--shell-safe-area-bottom));");
    expect(styles).toContain("scroll-padding-block: 1rem calc(var(--shell-navigation-space) + var(--shell-safe-area-bottom));");
    expect(styles).toContain("padding: .35rem .5rem var(--shell-safe-area-bottom);");
  });

  it("keeps nonzero inset overrides in the browser fixture entry", () => {
    expect(fixtureHTML).toContain("/src/browser-fixture.tsx");
    expect(browserFixtureSource).toContain('"--shell-safe-area-top": "24px"');
    expect(browserFixtureSource).toContain('"--shell-safe-area-bottom": "32px"');
    expect(styles).not.toContain("--shell-safe-area-top: 24px");
    expect(styles).not.toContain("--shell-safe-area-bottom: 32px");
  });
});

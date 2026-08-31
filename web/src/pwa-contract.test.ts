import { describe, expect, it } from "vitest";
import indexHTML from "../index.html?raw";
import manifestText from "../public/manifest.webmanifest?raw";
import icon180 from "../public/icons/icon-180.png?inline";
import icon192 from "../public/icons/icon-192.png?inline";
import icon512 from "../public/icons/icon-512.png?inline";

function pngDimensions(dataURL: string) {
  const encoded = dataURL.slice(dataURL.indexOf(",") + 1);
  const png = Uint8Array.from(atob(encoded), (byte) => byte.charCodeAt(0));
  return {
    width: png.slice(16, 20).reduce((value, byte) => value * 256 + byte, 0),
    height: png.slice(20, 24).reduce((value, byte) => value * 256 + byte, 0),
  };
}

describe("standalone PWA contract", () => {
  it("publishes install metadata so mobile browsers can create a Vibe MUD shortcut", () => {
    const manifest = JSON.parse(manifestText) as {
      name: string;
      short_name: string;
      start_url: string;
      scope: string;
      display: string;
      icons: Array<{ src: string; sizes: string; purpose?: string }>;
    };

    expect(manifest.name).toBe("Vibe MUD");
    expect(manifest.short_name).toBe("Vibe MUD");
    expect(manifest.start_url).toBe("/");
    expect(manifest.scope).toBe("/");
    expect(manifest.display).toBe("standalone");
    expect(manifest.icons).toEqual(expect.arrayContaining([
      expect.objectContaining({ src: "/icons/icon-192.png", sizes: "192x192" }),
      expect.objectContaining({ src: "/icons/icon-512.png", sizes: "512x512", purpose: "any maskable" }),
    ]));
  });

  it("keeps the iOS home-screen contract in the production entry document", () => {
    expect(indexHTML).toContain('<link rel="manifest" href="/manifest.webmanifest" />');
    expect(indexHTML).toContain('<meta name="theme-color" content="#101827" />');
    expect(indexHTML).toContain('<meta name="apple-mobile-web-app-capable" content="yes" />');
    expect(indexHTML).toContain('<meta name="apple-mobile-web-app-title" content="Vibe MUD" />');
    expect(indexHTML).toContain('<meta name="apple-mobile-web-app-status-bar-style" content="black-translucent" />');
    expect(indexHTML).toContain('<link rel="apple-touch-icon" sizes="180x180" href="/icons/icon-180.png" />');
  });

  it("ships exact icon dimensions required by the declared mobile targets", () => {
    expect(pngDimensions(icon180)).toEqual({ width: 180, height: 180 });
    expect(pngDimensions(icon192)).toEqual({ width: 192, height: 192 });
    expect(pngDimensions(icon512)).toEqual({ width: 512, height: 512 });
  });

  it("does not register a Service Worker because this MVP has no offline runtime", () => {
    expect(indexHTML).not.toMatch(/serviceWorker|service-worker|navigator\.serviceWorker/i);
  });
});

import { afterEach, describe, expect, it } from "vitest";
import {
  BASEMAP_IDS,
  BASEMAP_SOURCE_ID,
  BASEMAP_STORAGE_KEY,
  DEFAULT_BASEMAP,
  OFFLINE_STYLE,
  basemapStyle,
  isBasemapId,
  readStoredBasemap,
  storeBasemap,
} from "./basemap";
import { DEFAULT_TILE_URL, setTileURLOverride } from "./tile-source";

/**
 * The styles are built on demand rather than frozen at module load, because
 * the tile URL is a per-browser setting. These assertions are what makes that
 * true: a style captured at import time would pass the first one and fail the
 * second.
 */

afterEach(() => {
  localStorage.clear();
});

describe("basemapStyle", () => {
  it("builds every basemap on the default tile server", () => {
    for (const id of BASEMAP_IDS) {
      const style = basemapStyle(id);
      const source = style.sources[BASEMAP_SOURCE_ID];
      expect(source).toMatchObject({ tiles: [DEFAULT_TILE_URL] });
    }
  });

  it("reads the stored tile URL at build time, not at import time", () => {
    setTileURLOverride("https://tiles.internal/{z}/{x}/{y}.png");
    for (const id of BASEMAP_IDS) {
      expect(basemapStyle(id).sources[BASEMAP_SOURCE_ID]).toMatchObject({
        tiles: ["https://tiles.internal/{z}/{x}/{y}.png"],
      });
    }
  });

  it("takes an explicit tile URL over the stored one", () => {
    setTileURLOverride("https://stored/{z}/{x}/{y}.png");
    expect(
      basemapStyle("light", "https://explicit/{z}/{x}/{y}.png").sources[
        BASEMAP_SOURCE_ID
      ],
    ).toMatchObject({ tiles: ["https://explicit/{z}/{x}/{y}.png"] });
  });

  it("gives each basemap its own background and paint", () => {
    expect(basemapStyle("light").name).toBe("osm-light");
    expect(basemapStyle("dark").name).toBe("osm-dark");
    expect(basemapStyle("light")).not.toEqual(basemapStyle("dark"));
  });
});

describe("OFFLINE_STYLE", () => {
  it("requests no tiles at all", () => {
    // The whole point of the fallback: a map that keeps painting when the
    // network cannot be reached must not keep asking it for tiles.
    expect(OFFLINE_STYLE.sources).toEqual({});
  });
});

describe("the stored basemap choice", () => {
  it("defaults when nothing is stored", () => {
    expect(readStoredBasemap()).toBe(DEFAULT_BASEMAP);
  });

  it("round-trips a stored choice", () => {
    storeBasemap("dark");
    expect(readStoredBasemap()).toBe("dark");
  });

  it("ignores a value that is not a known basemap", () => {
    // A key left by an older build would otherwise index the recipes with
    // `undefined` and take the map down on the next build.
    localStorage.setItem(BASEMAP_STORAGE_KEY, "satellite");
    expect(readStoredBasemap()).toBe(DEFAULT_BASEMAP);
    expect(isBasemapId("satellite")).toBe(false);
    expect(isBasemapId("dark")).toBe(true);
  });
});

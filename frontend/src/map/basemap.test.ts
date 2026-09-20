import { afterEach, describe, expect, it } from "vitest";
import type { BasemapId } from "./basemap";
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

/** The raster layer's paint, which is the only thing that differs on screen. */
function rasterPaint(id: BasemapId): Record<string, unknown> {
  const layer = basemapStyle(id).layers.find((l) => l.id === "osm-raster");
  expect(layer, `${id} has no raster layer`).toBeDefined();
  return (layer?.paint ?? {}) as Record<string, unknown>;
}

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

  /**
   * Every basemap draws the same tiles, so the raster paint is the whole of
   * what tells them apart on screen. This pair of assertions is the only thing
   * that holds them visibly distinct: `light` once carried no paint at all and
   * `bright` a 0.1 saturation nudge over it, which passed the test above — the
   * two styles differ as objects — while looking like one basemap offered
   * twice. Comparing against `dark` alone never caught it.
   */
  it("paints every basemap, and no two of them alike", () => {
    for (const id of BASEMAP_IDS) {
      const paint = rasterPaint(id);
      expect(
        Object.keys(paint).length,
        `${id} carries no raster paint`,
      ).toBeGreaterThan(0);
    }

    for (const id of BASEMAP_IDS) {
      for (const other of BASEMAP_IDS) {
        if (id === other) continue;
        expect(rasterPaint(id), `${id} and ${other} paint alike`).not.toEqual(
          rasterPaint(other),
        );
      }
    }
  });

  /**
   * The direction each name promises, not only that the numbers differ.
   *
   * `light` is the backdrop the result contours are read over, so it has to
   * sit back from the raw tiles; `bright` is for reading the map itself, so it
   * has to sit forward of them. Swapping the two would keep the test above
   * green and put the loud basemap under every overlay.
   */
  it("pushes light back from the raw tiles and bright forward", () => {
    const light = rasterPaint("light");
    const bright = rasterPaint("bright");

    expect(light["raster-saturation"]).toBeLessThan(0);
    expect(light["raster-brightness-min"]).toBeGreaterThan(0);

    expect(bright["raster-saturation"]).toBeGreaterThan(0);
    expect(bright["raster-contrast"]).toBeGreaterThan(0);
  });

  /**
   * `light` has to sit back far enough to be seen doing it.
   *
   * `bright` deliberately barely touches the tiles — the tile server already
   * renders a map meant to be read — so the whole gap between the two now
   * rests on `light`'s black-point lift. A token value there would keep every
   * other assertion in this file green and hand back the two-names-one-picture
   * bug these tests exist to catch, so the floor is asserted rather than
   * implied. It is well under the value in use; this guards a collapse, not a
   * tweak.
   */
  it("lifts light far enough that the lift is what separates it", () => {
    const lift = rasterPaint("light")["raster-brightness-min"];

    expect(typeof lift).toBe("number");
    expect(lift).toBeGreaterThanOrEqual(0.2);
  });

  /**
   * The other half of that bargain: `bright` has to stay near the tiles.
   *
   * The test above says `bright` sits forward of the raw tiles, which any
   * positive number satisfies — including the 0.45/0.18 it briefly carried,
   * loud enough that the basemap competed with the model geometry and the
   * result contours drawn over it. Being forward of the tiles is the
   * direction; this is the distance.
   *
   * A ceiling rather than the exact pair, because the constraint is restraint
   * and not a particular number: the tile server already renders a map meant
   * to be read, and `bright` is the choice for reading it, so its paint is a
   * touch on that rendering rather than a replacement of it. The ceiling sits
   * well above the values in use and well below the ones that were rejected.
   */
  it("keeps bright near the tiles rather than shouting over them", () => {
    const bright = rasterPaint("bright");

    expect(bright["raster-saturation"]).toBeLessThanOrEqual(0.2);
    expect(bright["raster-contrast"]).toBeLessThanOrEqual(0.1);
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

  it("ignores an inherited object key stored as a basemap", () => {
    // `"constructor" in BASEMAP_RECIPES` is true, so an `in` check would take
    // `Object.prototype.constructor` for a recipe and build a style with no
    // background colour rather than falling back.
    localStorage.setItem(BASEMAP_STORAGE_KEY, "constructor");
    expect(readStoredBasemap()).toBe(DEFAULT_BASEMAP);
    expect(isBasemapId("constructor")).toBe(false);
    expect(isBasemapId("toString")).toBe(false);
    expect(isBasemapId("__proto__")).toBe(false);
  });
});

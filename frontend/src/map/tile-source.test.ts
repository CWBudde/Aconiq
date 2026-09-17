import { afterEach, describe, expect, it, vi } from "vitest";
import {
  DEFAULT_TILE_URL,
  TILE_URL_OVERRIDE_KEY,
  clearTileURLOverride,
  getTileURL,
  hasTileURLOverride,
  setTileURLOverride,
} from "./tile-source";

/**
 * The override is the air-gapped install's only way to point the map at a tile
 * server it can actually reach, and it is read on the path that builds the
 * map. So the contract is: a stored value wins, anything blank or unreadable
 * falls back to the default, and no access ever throws.
 */

afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

describe("tile URL override", () => {
  it("falls back to the OSM default when nothing is stored", () => {
    expect(getTileURL()).toBe(DEFAULT_TILE_URL);
    expect(hasTileURLOverride()).toBe(false);
  });

  it("returns a stored override", () => {
    setTileURLOverride("https://tiles.internal/{z}/{x}/{y}.png");
    expect(getTileURL()).toBe("https://tiles.internal/{z}/{x}/{y}.png");
    expect(hasTileURLOverride()).toBe(true);
  });

  it("trims but does not otherwise rewrite the template", () => {
    // Deliberately unlike the API base URL, which has its trailing slash
    // stripped: a tile template's last character is load-bearing.
    setTileURLOverride("  https://tiles.internal/{z}/{x}/{y}/  ");
    expect(localStorage.getItem(TILE_URL_OVERRIDE_KEY)).toBe(
      "https://tiles.internal/{z}/{x}/{y}/",
    );
  });

  it("treats an empty value as a removal", () => {
    setTileURLOverride("https://tiles.internal/{z}/{x}/{y}.png");
    setTileURLOverride("   ");
    expect(localStorage.getItem(TILE_URL_OVERRIDE_KEY)).toBeNull();
    expect(getTileURL()).toBe(DEFAULT_TILE_URL);
  });

  it("treats a blank stored value as no override", () => {
    // Nothing in the app writes this, but another tab or an older build might.
    localStorage.setItem(TILE_URL_OVERRIDE_KEY, "   ");
    expect(hasTileURLOverride()).toBe(false);
    expect(getTileURL()).toBe(DEFAULT_TILE_URL);
  });

  it("clears the override", () => {
    setTileURLOverride("https://tiles.internal/{z}/{x}/{y}.png");
    clearTileURLOverride();
    expect(hasTileURLOverride()).toBe(false);
    expect(getTileURL()).toBe(DEFAULT_TILE_URL);
  });

  it("survives storage that throws", () => {
    // Private mode, a blocked origin, a full quota: the map still has to be
    // built, so every access swallows and the default stands.
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new Error("access denied");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new Error("access denied");
    });
    vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => {
      throw new Error("access denied");
    });

    expect(getTileURL()).toBe(DEFAULT_TILE_URL);
    expect(hasTileURLOverride()).toBe(false);
    expect(() => {
      setTileURLOverride("https://tiles.internal/{z}/{x}/{y}.png");
    }).not.toThrow();
    expect(() => {
      setTileURLOverride("");
    }).not.toThrow();
    expect(() => {
      clearTileURLOverride();
    }).not.toThrow();
  });
});

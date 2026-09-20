import { afterEach, describe, expect, it, vi } from "vitest";
import {
  LAYER_VISIBILITY_STORAGE_KEY,
  RESULT_INDICATOR_STORAGE_KEY,
  readStoredLayerVisibility,
  readStoredResultIndicator,
  storeLayerVisibility,
  storeResultIndicator,
} from "./map-preferences";
import { MODEL_LAYER_GROUPS } from "./layers";

afterEach(() => {
  localStorage.clear();
  vi.restoreAllMocks();
});

/**
 * A group id that certainly exists, taken from the definitions themselves
 * rather than spelled out — a literal here would keep passing after the group
 * it names was renamed, which is the one thing these tests are about.
 */
const KNOWN_GROUP = firstModelGroupId();

function firstModelGroupId(): string {
  const [group] = MODEL_LAYER_GROUPS;
  if (!group) throw new Error("no model layer groups to test against");
  return group.id;
}

describe("the stored layer visibility", () => {
  it("is empty when nothing is stored", () => {
    expect(readStoredLayerVisibility()).toEqual({});
  });

  it("round-trips a hidden group", () => {
    storeLayerVisibility({ [KNOWN_GROUP]: false });
    expect(readStoredLayerVisibility()).toEqual({ [KNOWN_GROUP]: false });
  });

  it("stores nothing at all for the default", () => {
    // An empty record *is* the default, so writing one would leave a key
    // behind that says exactly what its absence says.
    storeLayerVisibility({ [KNOWN_GROUP]: false });
    storeLayerVisibility({});

    expect(localStorage.getItem(LAYER_VISIBILITY_STORAGE_KEY)).toBeNull();
    expect(readStoredLayerVisibility()).toEqual({});
  });

  it("drops a group this build no longer has", () => {
    // A renamed or removed group would otherwise keep its entry forever, and
    // the key would grow with every rename the app ever goes through.
    localStorage.setItem(
      LAYER_VISIBILITY_STORAGE_KEY,
      JSON.stringify({ [KNOWN_GROUP]: false, "retired-group": false }),
    );

    expect(readStoredLayerVisibility()).toEqual({ [KNOWN_GROUP]: false });
  });

  it("drops a value that is not a boolean", () => {
    // This ends up deciding `"visible"` or `"none"` for `setLayoutProperty`;
    // a hand-edited string must not reach it.
    localStorage.setItem(
      LAYER_VISIBILITY_STORAGE_KEY,
      JSON.stringify({ [KNOWN_GROUP]: "none" }),
    );

    expect(readStoredLayerVisibility()).toEqual({});
  });

  it("falls back when the value is not JSON, or not an object", () => {
    localStorage.setItem(LAYER_VISIBILITY_STORAGE_KEY, "{not json");
    expect(readStoredLayerVisibility()).toEqual({});

    localStorage.setItem(LAYER_VISIBILITY_STORAGE_KEY, JSON.stringify([1, 2]));
    expect(readStoredLayerVisibility()).toEqual({});

    localStorage.setItem(LAYER_VISIBILITY_STORAGE_KEY, JSON.stringify(null));
    expect(readStoredLayerVisibility()).toEqual({});
  });
});

describe("the stored result indicator", () => {
  it("is null when nothing is stored", () => {
    expect(readStoredResultIndicator()).toBeNull();
  });

  it("round-trips a band name", () => {
    storeResultIndicator("Lden");
    expect(readStoredResultIndicator()).toBe("Lden");
  });

  it("treats null and blank as no preference", () => {
    storeResultIndicator("Lden");
    storeResultIndicator(null);
    expect(localStorage.getItem(RESULT_INDICATOR_STORAGE_KEY)).toBeNull();

    storeResultIndicator("Lden");
    storeResultIndicator("   ");
    expect(readStoredResultIndicator()).toBeNull();
  });

  it("does not validate the band against anything", () => {
    // Deliberate: which bands exist is a property of a run's receiver table,
    // which has not arrived when this is read. `ResultLayers` resolves it.
    storeResultIndicator("a-band-no-run-has");
    expect(readStoredResultIndicator()).toBe("a-band-no-run-has");
  });
});

describe("a browser with storage disabled", () => {
  // Nothing on the path that builds a map may throw. `tile-source.test.ts`
  // makes the same case for the same reason.
  it("reads defaults and swallows writes", () => {
    const boom = () => {
      throw new Error("storage disabled");
    };
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(boom);
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(boom);
    vi.spyOn(Storage.prototype, "removeItem").mockImplementation(boom);

    expect(readStoredLayerVisibility()).toEqual({});
    expect(readStoredResultIndicator()).toBeNull();
    expect(() => {
      storeLayerVisibility({ [KNOWN_GROUP]: false });
    }).not.toThrow();
    expect(() => {
      storeLayerVisibility({});
    }).not.toThrow();
    expect(() => {
      storeResultIndicator("Lden");
    }).not.toThrow();
    expect(() => {
      storeResultIndicator(null);
    }).not.toThrow();
  });
});

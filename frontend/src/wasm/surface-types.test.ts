import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { RLS19_SURFACE_TYPES } from "@/model/source-acoustics";
import type { ProfileInfo } from "@/standards/descriptor";
import { getNodeKernel, kernelSkipReason } from "./kernel-node";

const skipReason = kernelSkipReason();

/**
 * The RLS-19 road surface list exists in three places:
 *
 *   1. Go constants in `backend/internal/standards/rls19/road/model.go`
 *      (normative — each value has its own DStrO correction row in tables.go),
 *   2. the `SurfaceType` union in `src/wasm/types.ts` (the WASM call contract),
 *   3. `RLS19_SURFACE_TYPES` in `src/model/source-acoustics.ts` (the picker).
 *
 * All three have been completed by hand at different times and have drifted
 * before. A missing value is a numeric defect, not a cosmetic one: the kernel
 * silently applies a different surface correction than the UI offered. These
 * tests read the Go constants as the source of truth and fail on any divergence.
 */

const repoFile = (relative: string): string =>
  readFileSync(fileURLToPath(new URL(relative, import.meta.url)), "utf8");

/** Surface identifiers declared as Go constants, in declaration order. */
function goSurfaceTypes(): string[] {
  const source = repoFile(
    "../../../backend/internal/standards/rls19/road/model.go",
  );
  const values: string[] = [];
  const pattern = /^\s*Surface\w+\s+SurfaceType\s*=\s*"([^"]*)"/gm;
  for (const match of source.matchAll(pattern)) {
    values.push(match[1] ?? "");
  }
  return values;
}

/** String literals of the `SurfaceType` union in `src/wasm/types.ts`. */
function wasmSurfaceTypes(): string[] {
  const source = repoFile("./types.ts");
  const union = /export type SurfaceType\s*=([\s\S]*?);/.exec(source);
  expect(
    union,
    "SurfaceType union not found in src/wasm/types.ts",
  ).not.toBeNull();
  const body = union?.[1] ?? "";
  return [...body.matchAll(/"([^"]*)"/g)].map((match) => match[1] ?? "");
}

describe("RLS-19 SurfaceType lists", () => {
  it("finds the Go constants", () => {
    // Guards the regexes themselves: a rename in the Go file must fail loudly
    // here rather than turn the comparisons below into vacuous truths.
    const values = goSurfaceTypes();
    expect(values.length).toBeGreaterThan(10);
    expect(values).toContain("");
    expect(values).toContain("SMA");
    expect(values).toContain("beschaedigt");
  });

  it("matches the wasm/types.ts SurfaceType union exactly", () => {
    expect(wasmSurfaceTypes()).toEqual(goSurfaceTypes());
  });

  it("matches RLS19_SURFACE_TYPES apart from the empty 'not specified' value", () => {
    // The picker offers only selectable surfaces; "" means "not specified" and
    // is the field's absent state, not an option.
    const selectable = goSurfaceTypes().filter((value) => value !== "");
    expect([...RLS19_SURFACE_TYPES]).toEqual(selectable);
  });
});

/** `supported_source_types` of the Go rls19-road descriptor. */
function goSupportedSourceTypes(): string[] {
  const source = repoFile(
    "../../../backend/internal/standards/rls19/road/model.go",
  );
  const match = /SupportedSourceTypes:\s*\[\]string\{([^}]*)\}/.exec(source);
  expect(
    match,
    "SupportedSourceTypes not found in rls19/road/model.go",
  ).not.toBeNull();
  return [...(match?.[1] ?? "").matchAll(/"([^"]*)"/g)].map((m) => m[1] ?? "");
}

/**
 * `framework.UnitX` constant name → the symbol it stands for, from
 * `backend/internal/standards/framework/framework.go`. Spelled out here so a
 * renamed or retired constant fails loudly rather than silently dropping a
 * parameter from the comparison below.
 */
const GO_UNIT_SYMBOLS: Record<string, string> = {
  UnitMeter: "m",
  UnitKilometersPerHour: "km/h",
  UnitDecibel: "dB",
  UnitDecibelPerKilometer: "dB/km",
  UnitPerHour: "1/h",
  UnitPerKilometer: "1/km",
  UnitPercent: "%",
  UnitDegreeCelsius: "°C",
  UnitDegree: "°",
};

/** Parameter name → unit symbol, as declared by the Go rls19-road descriptor. */
function goParameterUnits(): Record<string, string> {
  const source = repoFile(
    "../../../backend/internal/standards/rls19/road/model.go",
  );
  const units: Record<string, string> = {};
  const pattern =
    /\{Name:\s*"([a-z0-9_]+)",[^}]*?Unit:\s*framework\.(Unit\w+)/g;
  for (const match of source.matchAll(pattern)) {
    const constant = match[2] ?? "";
    const symbol = GO_UNIT_SYMBOLS[constant];
    expect(
      symbol,
      `unknown framework unit constant ${constant} — add it to GO_UNIT_SYMBOLS`,
    ).toBeDefined();
    units[match[1] ?? ""] = symbol ?? "";
  }
  return units;
}

/**
 * The descriptor browser mode runs against, as the kernel publishes it.
 *
 * It used to be a hand-maintained TypeScript copy of the Go descriptor, and it
 * had drifted: before these tests it offered 9 of the 17 surfaces and claimed
 * to support line sources only, long after the Go module started accepting
 * `area` features as Parkplätze. The copy is gone — `aconiq.standards()` now
 * publishes the module's own descriptor — so what these tests check is no
 * longer a second list but the crossing itself: that the Go source's
 * declarations survive `descriptorjson` and the WASM boundary intact.
 */
async function kernelRoadProfile(): Promise<ProfileInfo> {
  const standards = (await getNodeKernel()).standards();
  const road = standards.find((standard) => standard.id === "rls19-road");
  expect(road, "the kernel publishes no rls19-road descriptor").toBeDefined();
  const profile = road?.versions[0]?.profiles[0];
  expect(profile, "rls19-road default profile not found").toBeDefined();
  // Asserted just above; the cast keeps every caller free of `?.` chains.
  return profile as ProfileInfo;
}

describe.skipIf(skipReason !== null)("kernel rls19-road descriptor", () => {
  it("supports the same source types the Go descriptor does", async () => {
    const profile = await kernelRoadProfile();
    expect([...profile.supported_source_types].sort()).toEqual(
      [...goSupportedSourceTypes()].sort(),
    );
  });

  it("offers every selectable surface, not a subset", async () => {
    const profile = await kernelRoadProfile();
    const surface = profile.parameters.find((p) => p.name === "surface_type");
    expect(surface?.enum).toEqual([...RLS19_SURFACE_TYPES]);
  });

  it("carries the evidence tier the Go module declares", async () => {
    const standards = (await getNodeKernel()).standards();
    const road = standards.find((standard) => standard.id === "rls19-road");
    // Priority 4's whole point: the tier travels with the descriptor instead
    // of being declared a second time in the frontend, where it could drift.
    expect(road?.evidence_tier).toBe("normative");
    expect(road?.context).toBe("planning");
  });

  it("advertises only what the kernel can run", async () => {
    // A kernel listing the whole registry would offer twelve standards it has
    // no entry point for — the dishonesty the evidence tier exists to prevent.
    const standards = (await getNodeKernel()).standards();
    expect(standards.map((standard) => standard.id)).toEqual(["rls19-road"]);
  });
});

/**
 * `unit` is published by both backends, so HTTP mode gets it from the Go
 * descriptor for free. This checks the kernel does too — without it the same
 * parameter could carry a unit in HTTP mode and none in browser mode, and a
 * parameter form reading the shared contract would have to keep its own
 * name-to-unit table, which is exactly what declaring the unit made
 * unnecessary.
 */
describe.skipIf(skipReason !== null)(
  "kernel rls19-road parameter units",
  () => {
    it("finds the Go unit declarations", () => {
      // Guards the regex: a change to the Go declaration style must fail here
      // rather than turn the comparison below into a vacuous truth.
      const units = goParameterUnits();
      expect(Object.keys(units).length).toBeGreaterThan(10);
      expect(units["speed_pkw_kph"]).toBe("km/h");
      expect(units["traffic_day_lkw1"]).toBe("1/h");
    });

    it("declares the same unit the Go descriptor does, for every parameter", async () => {
      const profile = await kernelRoadProfile();
      const units: Record<string, string> = {};
      for (const parameter of profile.parameters) {
        if (parameter.unit !== undefined)
          units[parameter.name] = parameter.unit;
      }
      expect(units).toEqual(goParameterUnits());
    });

    it("leaves dimensionless and non-numeric parameters unitless", async () => {
      // surface_type is an enum: a unit on it would be a spelling mistake, and
      // toEqual above only catches that because Go declares none either.
      const profile = await kernelRoadProfile();
      const surface = profile.parameters.find((p) => p.name === "surface_type");
      expect(surface?.unit).toBeUndefined();
    });
  },
);

if (skipReason !== null) {
  describe("kernel rls19-road descriptor", () => {
    it.skip(`skipped: ${skipReason}`, () => {
      // Recorded so the skip carries its reason into the runner output.
    });
  });
}

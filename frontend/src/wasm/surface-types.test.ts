import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { BROWSER_STANDARDS } from "@/api/browser-backend";
import { RLS19_SURFACE_TYPES } from "@/model/source-acoustics";

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

/** One parameter's `enum` from the hardcoded browser descriptor. */
function browserParameterEnum(name: string): string[] {
  const profile = BROWSER_STANDARDS[0]?.versions[0]?.profiles[0];
  const parameter = profile?.parameters.find((p) => p.name === name);
  expect(
    parameter,
    `parameter ${name} not found in BROWSER_STANDARDS`,
  ).toBeDefined();
  return [...(parameter?.enum ?? [])];
}

/**
 * The browser declares its own `rls19-road` descriptor because the kernel does
 * not expose one yet. It is therefore a hand-maintained copy of the Go
 * descriptor, and it has drifted: before these tests it offered 9 of the 17
 * surfaces and claimed to support line sources only, long after the Go module
 * started accepting `area` features as Parkplätze.
 */
describe("browser rls19-road descriptor", () => {
  it("supports the same source types the Go descriptor does", () => {
    const profile = BROWSER_STANDARDS[0]?.versions[0]?.profiles[0];
    expect([...(profile?.supported_source_types ?? [])].sort()).toEqual(
      [...goSupportedSourceTypes()].sort(),
    );
  });

  it("offers every selectable surface, not a subset", () => {
    expect(browserParameterEnum("surface_type")).toEqual([
      ...RLS19_SURFACE_TYPES,
    ]);
  });
});

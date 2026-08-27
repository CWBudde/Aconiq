import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
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

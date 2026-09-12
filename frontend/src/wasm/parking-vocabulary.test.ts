import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  PROP_PARKING_FACILITY_TYPE,
  PROP_PARKING_MOVEMENTS_DAY,
  PROP_PARKING_MOVEMENTS_NIGHT,
  PROP_PARKING_NUM_SPACES,
  PROP_PARKING_TYPE,
  RLS19_PARKING_FACILITY_RATES,
  RLS19_PARKING_FACILITY_TYPES,
  RLS19_PARKING_LOT_TYPES,
} from "@/model/rls19-parking";

/**
 * Browser mode and the CLI read the same model into the same kernel, so their
 * two extractors have to agree on every name. They are written in different
 * languages and completed at different times — which is exactly how the
 * descriptor's surface list drifted to 9 of 17 values without anyone noticing,
 * because surface-types.test.ts pinned the other two copies and not that one.
 *
 * These tests read the Go source as the source of truth. A vocabulary that
 * diverges is not cosmetic: an unrecognised Parkplatztyp costs up to 10 dB of
 * Tabelle 6 surcharge, and a property key the browser spells differently drops
 * a required input on the floor.
 */

const repoFile = (relative: string): string =>
  readFileSync(fileURLToPath(new URL(relative, import.meta.url)), "utf8");

const parkingGo = (): string =>
  repoFile("../../../backend/internal/standards/rls19/road/parking.go");

/** Constant values of one Go string enum, in declaration order. */
function goStringEnum(source: string, typeName: string): string[] {
  const values: string[] = [];
  const pattern = new RegExp(
    String.raw`^\s*\w+\s+${typeName}\s*=\s*"([^"]*)"`,
    "gm",
  );
  for (const match of source.matchAll(pattern)) {
    values.push(match[1] ?? "");
  }
  return values;
}

/** Selectable values: the "" zero value is the absent state, not a table row. */
const selectable = (values: string[]): string[] =>
  values.filter((value) => value !== "");

describe("RLS-19 Parkplatztyp vocabularies", () => {
  it("finds the Go constants", () => {
    // Guards the regexes: a rename in parking.go must fail here rather than
    // turn the comparisons below into vacuous truths.
    const lot = goStringEnum(parkingGo(), "ParkingLotType");
    expect(lot).toContain("");
    expect(lot).toContain("pkw");

    const facility = goStringEnum(parkingGo(), "ParkingFacilityType");
    expect(facility).toContain("");
    expect(facility).toContain("park-and-ride");
  });

  it("matches Tabelle 6 exactly", () => {
    expect([...RLS19_PARKING_LOT_TYPES]).toEqual(
      selectable(goStringEnum(parkingGo(), "ParkingLotType")),
    );
  });

  it("matches Tabelle 7 exactly", () => {
    expect([...RLS19_PARKING_FACILITY_TYPES]).toEqual(
      selectable(goStringEnum(parkingGo(), "ParkingFacilityType")),
    );
  });

  it("carries the Tabelle 7 movement rates the Go table declares", () => {
    const table =
      /parkingMovementRates = \[\]struct \{[\s\S]*?\n\}\{([\s\S]*?)\n\}/.exec(
        parkingGo(),
      );
    expect(
      table,
      "parkingMovementRates not found in parking.go",
    ).not.toBeNull();

    const rows = [
      ...(table?.[1] ?? "").matchAll(
        /\{\s*\w+\s*,\s*([0-9.]+)\s*,\s*([0-9.]+)\s*\}/g,
      ),
    ].map((match) => ({
      day: Number(match[1]),
      night: Number(match[2]),
    }));

    expect(rows).toHaveLength(RLS19_PARKING_FACILITY_RATES.length);
    expect(
      RLS19_PARKING_FACILITY_RATES.map((row) => ({
        day: row.day,
        night: row.night,
      })),
    ).toEqual(rows);
  });
});

describe("RLS-19 Parkplatz property keys", () => {
  it("spells every key the way the CLI extractor does", () => {
    const source = repoFile(
      "../../../backend/internal/app/cli/run_extract_rls19_parking.go",
    );
    const goKeys = [
      ...source.matchAll(/^\s*propRLS19Parking\w+\s*=\s*"([^"]*)"/gm),
    ].map((match) => match[1] ?? "");

    expect(goKeys.length).toBeGreaterThan(0);
    expect([...goKeys].sort()).toEqual(
      [
        PROP_PARKING_NUM_SPACES,
        PROP_PARKING_TYPE,
        PROP_PARKING_FACILITY_TYPE,
        PROP_PARKING_MOVEMENTS_DAY,
        PROP_PARKING_MOVEMENTS_NIGHT,
      ].sort(),
    );
  });
});

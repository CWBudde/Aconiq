import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import { describe, expect, it } from "vitest";
import type { ParameterDefinition } from "@/api/client";
import { m } from "@/i18n/messages";
import {
  CATALOGUED_STANDARD_DESCRIPTIONS,
  getStandardDescription,
} from "./standards-meta";
import {
  hasCataloguedDescription,
  hasCataloguedLabel,
  parameterDescription,
  parameterLabel,
} from "./parameter-meta";

/**
 * Every parameter of a normative module has a German sentence.
 *
 * `parameterDescription` falls back to the backend's English for anything the
 * catalogue does not name. That fallback is deliberate for the ten scaffold
 * modules and unacceptable for the three normative ones, where it would put one
 * English line under a German label — the defect this file exists to keep
 * closed. Without a test, a parameter added to `rls19/road/model.go` next month
 * would simply start rendering in English and nobody would see it.
 *
 * The Go source is read as the source of truth, the way
 * `src/wasm/surface-types.test.ts` reads it: the alternative is a list of
 * parameter names maintained here, which is the drift the test is meant to
 * catch, written down a second time.
 */

const MODULES: { id: string; dir: string; file: string; count: number }[] = [
  {
    id: "rls19-road",
    dir: "rls19/road",
    file: "model.go",
    // The expected counts are the guard against a silently vacuous regex: a
    // changed declaration style drops matches, and a comparison over an empty
    // list passes. They are asserted, not just documented.
    count: 19,
  },
  { id: "schall03", dir: "schall03", file: "params.go", count: 20 },
  { id: "iso9613", dir: "iso9613", file: "params.go", count: 14 },
];

// Resolved against the test runner's root (frontend/) rather than
// `import.meta.url`, for the reason `locale-parity.test.ts` gives: the jsdom
// environment does not give this module a file: URL.
const modulePath = (dir: string, file = ""): string =>
  join(process.cwd(), "..", "backend", "internal", "standards", dir, file);

/**
 * `const Name = "value"` across the module's package.
 *
 * Two parameters are declared through a constant rather than a literal —
 * `schall03.ParamEngine` and `iso9613.paramMeteorologyAssumption` — and each is
 * spelled in a different file from the table that uses it. Resolving them
 * matters: skipping what the literal regex cannot read is how a gate quietly
 * stops covering the parameter most likely to be renamed.
 */
function packageConstants(dir: string): Record<string, string> {
  const constants: Record<string, string> = {};
  for (const entry of readdirSync(modulePath(dir))) {
    if (!entry.endsWith(".go") || entry.endsWith("_test.go")) continue;
    const source = readFileSync(modulePath(dir, entry), "utf8");
    for (const match of source.matchAll(
      /^\s*(?:const\s+)?([A-Za-z_][A-Za-z0-9_]*)\s*=\s*"([^"]*)"$/gm,
    )) {
      constants[match[1] ?? ""] = match[2] ?? "";
    }
  }
  return constants;
}

/**
 * The parameter names one module declares.
 *
 * `Kind:` is the discriminator. `Name:` on its own also introduces a version
 * ("2019") and a profile ("default"); only a `framework.ParameterDefinition`
 * puts a kind straight after the name.
 */
function goParameterNames(dir: string, file: string): string[] {
  const source = readFileSync(modulePath(dir, file), "utf8");
  const constants = packageConstants(dir);
  const names: string[] = [];
  for (const match of source.matchAll(
    /Name:\s*(?:"([^"]*)"|([A-Za-z_][A-Za-z0-9_]*))\s*,\s*Kind:/g,
  )) {
    const literal = match[1];
    if (literal !== undefined) {
      names.push(literal);
      continue;
    }
    const identifier = match[2] ?? "";
    const resolved = constants[identifier];
    expect(
      resolved,
      `${dir}: parameter name ${identifier} resolves to no string constant`,
    ).toBeDefined();
    names.push(resolved ?? "");
  }
  return names;
}

/**
 * `framework.Unit*` constant name → the symbol it stands for.
 *
 * Spelled out so a renamed constant fails loudly rather than silently dropping
 * a parameter's unit — the same table, for the same reason, as
 * `src/wasm/surface-types.test.ts`.
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

/**
 * Parameter name → declared unit, for the module's own table.
 *
 * The label check needs it: `parameterLabel` trims a trailing token only when
 * the parameter declares a unit, so asking whether the vehicle-class rule fires
 * for `speed_pkw_kph` without its `km/h` would answer a different question than
 * the render does.
 */
function goParameterUnits(dir: string, file: string): Record<string, string> {
  const source = readFileSync(modulePath(dir, file), "utf8");
  const units: Record<string, string> = {};
  for (const match of source.matchAll(
    /Name:\s*(?:"([^"]*)"|([A-Za-z_][A-Za-z0-9_]*))[^}]*?Unit:\s*framework\.(Unit\w+)/g,
  )) {
    const symbol = GO_UNIT_SYMBOLS[match[3] ?? ""];
    expect(
      symbol,
      `unknown framework unit constant ${match[3] ?? ""} — add it to GO_UNIT_SYMBOLS`,
    ).toBeDefined();
    units[match[1] ?? match[2] ?? ""] = symbol ?? "";
  }
  return units;
}

/** The shape `parameterDescription` reads; nothing else is needed here. */
function definition(name: string, unit?: string): ParameterDefinition {
  return {
    name,
    kind: "float",
    required: false,
    ...(unit === undefined ? {} : { unit }),
    // Deliberately recognisable: if a catalogue entry is missing, this is what
    // the assertion reports, instead of an English sentence that looks fine.
    description: `UNCATALOGUED ${name}`,
  };
}

describe.each(MODULES)(
  "$id parameter descriptions",
  ({ id, dir, file, count }) => {
    it("finds every parameter the Go module declares", () => {
      const names = goParameterNames(dir, file);
      expect(names).toHaveLength(count);
      expect(new Set(names).size).toBe(count);
    });

    it("has a catalogue sentence for every one of them", () => {
      const uncatalogued = goParameterNames(dir, file).filter(
        (name) => !hasCataloguedDescription(id, name),
      );
      expect(
        uncatalogued,
        "these parameters would render the backend's English under a German label",
      ).toEqual([]);
    });

    it("never falls back to the backend string", () => {
      for (const name of goParameterNames(dir, file)) {
        const rendered = parameterDescription(id, definition(name));
        expect(rendered, name).not.toBeNull();
        expect(rendered, name).not.toContain("UNCATALOGUED");
      }
    });

    it("has a catalogue label for every one of them", () => {
      // The other half of the same rule. German sentences under English labels
      // is the mix this change set out to remove, only upside down.
      const units = goParameterUnits(dir, file);
      const uncatalogued = goParameterNames(dir, file).filter(
        (name) => !hasCataloguedLabel(id, definition(name, units[name])),
      );
      expect(
        uncatalogued,
        "these parameters would render a humanised English name above a German sentence",
      ).toEqual([]);
    });

    it("never labels a parameter with its raw name, humanised", () => {
      const units = goParameterUnits(dir, file);
      for (const name of goParameterNames(dir, file)) {
        expect(
          parameterLabel(id, definition(name, units[name])),
          name,
        ).not.toBe("");
      }
    });
  },
);

describe("parameterDescription", () => {
  it("leaves a scaffold module's description as the backend wrote it", () => {
    // Whole, not half: the gate is on the standard, so a name the catalogue
    // happens to know does not get German inside a module whose other fields
    // cannot have any.
    expect(
      parameterDescription("cnossos-road", definition("grid_resolution_m")),
    ).toBe("UNCATALOGUED grid_resolution_m");
  });

  it("says nothing rather than rendering an empty line", () => {
    expect(
      parameterDescription("cnossos-road", {
        name: "whatever",
        kind: "float",
        required: false,
      }),
    ).toBeNull();
  });

  it("gives the per-source defaults one sentence", () => {
    // Twenty-five parameters share it, and it says more than "Night Pkw per
    // hour" did — the group, the label and the unit are already on screen.
    expect(
      parameterDescription("rls19-road", definition("traffic_night_pkw")),
    ).toBe(m.param_desc_source_default());
    expect(
      parameterDescription("schall03", definition("rail_track_type")),
    ).toBe(m.param_desc_source_default());
  });

  it("prefers a parameter's own sentence over the shared one", () => {
    expect(parameterDescription("rls19-road", definition("surface_type"))).toBe(
      m.param_desc_surface_type(),
    );
  });

  it("gives a name shared across modules one sentence, not three", () => {
    const own = parameterDescription(
      "rls19-road",
      definition("grid_resolution_m"),
    );
    expect(
      parameterDescription("schall03", definition("grid_resolution_m")),
    ).toBe(own);
    expect(
      parameterDescription("iso9613", definition("grid_resolution_m")),
    ).toBe(own);
  });
});

/**
 * A German entry copied from the English one renders, and `locale-parity`
 * cannot see it: the key is present in both files. This is the check
 * `validation-message.test.ts` makes for finding codes, applied to the
 * descriptions — the sentences are what this change is for.
 */
describe("parameter description catalogue", () => {
  const catalogue = (locale: "en" | "de"): Record<string, string> =>
    JSON.parse(
      readFileSync(join(process.cwd(), "messages", `${locale}.json`), "utf8"),
    ) as Record<string, string>;

  it("says something different in each locale", () => {
    const en = catalogue("en");
    const de = catalogue("de");
    const keys = Object.keys(en).filter(
      (k) =>
        k.startsWith("param_desc_") ||
        k.startsWith("param_label_") ||
        k.startsWith("standard_desc_"),
    );
    expect(keys.length).toBeGreaterThan(50);
    const untranslated = keys.filter((key) => en[key] === de[key]);
    expect(untranslated).toEqual([]);
  });
});

/**
 * The dialog's most prominent line is the standard's own description, and it
 * came from the same English descriptor the parameters did.
 */
describe("standard descriptions", () => {
  it("covers exactly the modules the parameter catalogue covers", () => {
    // One list, not two: a module whose parameters read German must not
    // introduce itself in English, and vice versa.
    expect([...CATALOGUED_STANDARD_DESCRIPTIONS].sort()).toEqual(
      MODULES.map((module) => module.id).sort(),
    );
  });

  it("never falls back to the descriptor for one of them", () => {
    for (const { id } of MODULES) {
      expect(getStandardDescription(id, "UNCATALOGUED"), id).not.toBe(
        "UNCATALOGUED",
      );
    }
  });

  it("leaves a scaffold's own words alone", () => {
    expect(getStandardDescription("cnossos-road", "UNCATALOGUED")).toBe(
      "UNCATALOGUED",
    );
  });
});

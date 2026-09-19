/**
 * Every code the validator can emit renders a sentence, in both locales.
 *
 * The `switch` in `validation-message.ts` is exhaustive by type, so a code
 * added to `ValidationIssueParams` without a branch does not compile. What the
 * compiler cannot see is the catalogue: `m.msg_validation_*` resolves against
 * `messages/{en,de}.json`, and a key missing from `de.json` silently falls back
 * to English — which is exactly the failure `locale-parity.test.ts` exists for
 * and exactly the one this file confirms at the rendering end.
 *
 * The fixtures below are one issue per code, built from the parameter shape the
 * type demands, so the list cannot fall behind the vocabulary: a new code with
 * no entry here is a missing key in `ISSUES` and a type error.
 */

import { describe, expect, it } from "vitest";
import { setLocale } from "@/i18n/runtime";
import type { ValidationCode, ValidationIssue } from "./types";
import { validationIssueText } from "./validation-message";

/** One representative finding per code, with realistic parameters. */
const ISSUES: {
  [Code in ValidationCode]: Extract<ValidationIssue, { code: Code }>;
} = {
  "model.empty": {
    level: "error",
    code: "model.empty",
    featureId: "",
    params: {},
  },
  "feature.id.duplicate": {
    level: "error",
    code: "feature.id.duplicate",
    featureId: "f1",
    params: {},
  },
  "receiver.id.duplicate": {
    level: "error",
    code: "receiver.id.duplicate",
    featureId: "r1",
    params: {},
  },
  "source.type.required": {
    level: "error",
    code: "source.type.required",
    featureId: "s1",
    params: {},
  },
  "source.geometry.mismatch": {
    level: "error",
    code: "source.geometry.mismatch",
    featureId: "s1",
    params: { geometry: "Point", sourceType: "line" },
  },
  "building.height.required": {
    level: "error",
    code: "building.height.required",
    featureId: "b1",
    params: {},
  },
  "building.height.invalid": {
    level: "error",
    code: "building.height.invalid",
    featureId: "b1",
    params: {},
  },
  "building.geometry.invalid": {
    level: "error",
    code: "building.geometry.invalid",
    featureId: "b1",
    params: {},
  },
  "barrier.height.required": {
    level: "error",
    code: "barrier.height.required",
    featureId: "w1",
    params: {},
  },
  "barrier.height.invalid": {
    level: "error",
    code: "barrier.height.invalid",
    featureId: "w1",
    params: {},
  },
  "barrier.geometry.invalid": {
    level: "error",
    code: "barrier.geometry.invalid",
    featureId: "w1",
    params: {},
  },
  "groundzone.factor.required": {
    level: "error",
    code: "groundzone.factor.required",
    featureId: "z1",
    params: {},
  },
  "groundzone.factor.invalid": {
    level: "error",
    code: "groundzone.factor.invalid",
    featureId: "z1",
    params: {},
  },
  "groundzone.geometry.invalid": {
    level: "error",
    code: "groundzone.geometry.invalid",
    featureId: "z1",
    params: {},
  },
  "receiver.coordinates.invalid": {
    level: "error",
    code: "receiver.coordinates.invalid",
    featureId: "r1",
    params: {},
  },
  "receiver.height.invalid": {
    level: "error",
    code: "receiver.height.invalid",
    featureId: "r1",
    params: {},
  },
  "source.rls19.surface_type.invalid": {
    level: "error",
    code: "source.rls19.surface_type.invalid",
    featureId: "s1",
    params: { value: "kopfsteinpflaster" },
  },
  "source.rls19.junction_type.invalid": {
    level: "error",
    code: "source.rls19.junction_type.invalid",
    featureId: "s1",
    params: { value: "kreisverkehr" },
  },
  "source.rls19.gradient.invalid": {
    level: "error",
    code: "source.rls19.gradient.invalid",
    featureId: "s1",
    params: { min: -12, max: 12 },
  },
  "source.rls19.junction_distance.invalid": {
    level: "error",
    code: "source.rls19.junction_distance.invalid",
    featureId: "s1",
    params: {},
  },
  "source.rls19.reflection_surcharge.invalid": {
    level: "error",
    code: "source.rls19.reflection_surcharge.invalid",
    featureId: "s1",
    params: {},
  },
  "source.rls19.road_speed.invalid": {
    level: "error",
    code: "source.rls19.road_speed.invalid",
    featureId: "s1",
    params: {},
  },
  "source.rls19.speed.invalid": {
    level: "error",
    code: "source.rls19.speed.invalid",
    featureId: "s1",
    params: { field: "speed_pkw_day_kph" },
  },
  "source.rls19.traffic.invalid": {
    level: "error",
    code: "source.rls19.traffic.invalid",
    featureId: "s1",
    params: { field: "traffic_day_lkw1" },
  },
  "source.rls19.review_required": {
    level: "warning",
    code: "source.rls19.review_required",
    featureId: "s1",
    params: {},
  },
  "source.rls19.parking.geometry.multipart": {
    level: "error",
    code: "source.rls19.parking.geometry.multipart",
    featureId: "p1",
    params: { parts: 2, field: "rls19_parking_num_spaces" },
  },
  "source.rls19.parking.num_spaces.missing": {
    level: "error",
    code: "source.rls19.parking.num_spaces.missing",
    featureId: "p1",
    params: { field: "rls19_parking_num_spaces" },
  },
  "source.rls19.parking.num_spaces.invalid": {
    level: "error",
    code: "source.rls19.parking.num_spaces.invalid",
    featureId: "p1",
    params: { field: "rls19_parking_num_spaces" },
  },
  "source.rls19.parking.parking_type.missing": {
    level: "error",
    code: "source.rls19.parking.parking_type.missing",
    featureId: "p1",
    params: { field: "rls19_parking_type", expected: "pkw, lkw" },
  },
  "source.rls19.parking.parking_type.invalid": {
    level: "error",
    code: "source.rls19.parking.parking_type.invalid",
    featureId: "p1",
    params: { value: "fahrrad" },
  },
  "source.rls19.parking.facility_type.invalid": {
    level: "error",
    code: "source.rls19.parking.facility_type.invalid",
    featureId: "p1",
    params: {
      field: "rls19_parking_facility_type",
      value: "flughafen",
      expected: "wohnhaus, kaufhaus",
    },
  },
  "source.rls19.parking.movements.missing": {
    level: "error",
    code: "source.rls19.parking.movements.missing",
    featureId: "p1",
    params: {
      field: "rls19_parking_movements_per_space_day",
      facilityField: "rls19_parking_facility_type",
    },
  },
  "source.rls19.parking.movements.invalid": {
    level: "error",
    code: "source.rls19.parking.movements.invalid",
    featureId: "p1",
    params: { field: "rls19_parking_movements_per_space_night" },
  },
};

const CODES = Object.keys(ISSUES) as ValidationCode[];

// `setLocale` reloads the page by default, which jsdom answers with a
// "Not implemented: navigation" error and no locale change at all. The promise
// it returns covers that reload; the locale itself is in place before it
// resolves, which is what makes the assertions below synchronous.
function withLocale(locale: "en" | "de"): void {
  void setLocale(locale, { reload: false });
}

describe("validationIssueText", () => {
  it.each(["en", "de"] as const)("renders every code in %s", (locale) => {
    withLocale(locale);

    const blank = CODES.filter(
      (code) => validationIssueText(ISSUES[code]).trim() === "",
    );

    expect(blank).toEqual([]);
  });

  it("says something different in each locale", () => {
    // Not cosmetic: a key present in `en.json` and missing from `de.json`
    // compiles, renders and returns the English string, so identical output is
    // the signature of the exact defect `locale-parity.test.ts` guards the
    // catalogue against. All thirty differ today, and the assertion is written
    // as an empty list rather than a count so a future exception has to be
    // named here rather than absorbed by a threshold.
    const sameInBoth = CODES.filter((code) => {
      withLocale("en");
      const en = validationIssueText(ISSUES[code]);
      withLocale("de");
      return validationIssueText(ISSUES[code]) === en;
    });

    expect(sameInBoth).toEqual([]);
  });

  it("interpolates the parameters rather than printing their names", () => {
    withLocale("en");

    expect(validationIssueText(ISSUES["source.geometry.mismatch"])).toContain(
      "Point",
    );
    expect(validationIssueText(ISSUES["source.geometry.mismatch"])).toContain(
      "line",
    );
    expect(
      validationIssueText(ISSUES["source.rls19.traffic.invalid"]),
    ).toContain("traffic_day_lkw1");
    expect(
      validationIssueText(ISSUES["source.rls19.gradient.invalid"]),
    ).toContain("-12");

    const multipart = validationIssueText(
      ISSUES["source.rls19.parking.geometry.multipart"],
    );
    expect(multipart).toContain("2");
    expect(multipart).toContain("Teilfläche");
    expect(multipart).not.toContain("{");
  });

  it("keeps the raw property name in German too", () => {
    // The parameter is what `--param` and the model file take, so it is not
    // translated on either side; the sentence around it is.
    withLocale("de");

    const text = validationIssueText(ISSUES["source.rls19.traffic.invalid"]);

    expect(text).toContain("traffic_day_lkw1");
    expect(text).not.toContain("{field}");
  });
});

import { describe, expect, it } from "vitest";
import type { ParameterDefinition } from "@/api/client";
import {
  PARAMETER_GROUP_ORDER,
  parameterGroup,
  parameterGroupLabel,
  parameterLabel,
  parameterUnitSuffix,
} from "./parameter-meta";
import { m } from "@/i18n/messages";

function param(
  name: string,
  unit?: string,
  extra: Partial<ParameterDefinition> = {},
): ParameterDefinition {
  return {
    name,
    kind: "float",
    required: false,
    ...(unit === undefined ? {} : { unit }),
    ...extra,
  };
}

describe("parameterGroup", () => {
  it.each([
    ["grid_resolution_m", "grid"],
    ["receiver_height_m", "grid"],
    ["speed_pkw_kph", "speeds"],
    ["traffic_day_lkw1", "traffic_day"],
    ["traffic_evening_pkw", "traffic_evening"],
    ["traffic_night_krad", "traffic_night"],
    ["surface_type", "other"],
    ["min_distance_m", "other"],
  ])("puts %s in %s", (name, group) => {
    expect(parameterGroup(name)).toBe(group);
  });

  it("puts a parameter no rule matches in the catch-all", () => {
    expect(parameterGroup("something_a_later_backend_adds")).toBe("other");
  });

  it("names every group it can return", () => {
    for (const key of PARAMETER_GROUP_ORDER) {
      expect(parameterGroupLabel(key)).not.toBe("");
    }
  });

  it("puts the catch-all last, so a topic never sorts after it", () => {
    expect(PARAMETER_GROUP_ORDER.at(-1)).toBe("other");
  });
});

describe("parameterLabel", () => {
  it("names a vehicle class by its class alone inside a period group", () => {
    // The group heading has already said which period it is, so repeating it
    // on every one of the four fields is noise.
    expect(parameterLabel(param("traffic_day_lkw1", "1/h"))).toBe(
      m.label_vehicle_class_lkw1(),
    );
    expect(parameterLabel(param("speed_pkw_kph", "km/h"))).toBe(
      m.label_vehicle_class_pkw(),
    );
  });

  it("does not shorten a grid parameter that happens to end in a class name", () => {
    expect(parameterLabel(param("grid_resolution_m", "m"))).toBe(
      "Grid resolution",
    );
  });

  it("humanises a name the catalogue has no terminology for", () => {
    expect(parameterLabel(param("surface_type"))).toBe("Surface type");
    expect(parameterLabel(param("aircraft_procedure_type"))).toBe(
      "Aircraft procedure type",
    );
  });

  it("drops a trailing token that only restates the declared unit", () => {
    expect(parameterLabel(param("min_distance_m", "m"))).toBe("Min distance");
    expect(parameterLabel(param("gradient_percent", "%"))).toBe("Gradient");
  });

  it("keeps a trailing token when nothing declares it a unit", () => {
    // `unit` is optional and older backends omit it; a name is never shortened
    // on the guess that its last token is a unit.
    expect(parameterLabel(param("min_distance_m"))).toBe("Min distance m");
  });

  it("never shortens a name to nothing", () => {
    expect(parameterLabel(param("m", "m"))).toBe("M");
  });

  it("spells an acronym the way it is written", () => {
    expect(parameterLabel(param("db_offset"))).toBe("dB offset");
  });
});

describe("parameterUnitSuffix", () => {
  it("reads the unit the descriptor publishes", () => {
    expect(parameterUnitSuffix(param("speed_pkw_kph", "km/h"))).toBe(" (km/h)");
  });

  it("says nothing for a dimensionless parameter", () => {
    expect(parameterUnitSuffix(param("surface_type"))).toBe("");
  });
});

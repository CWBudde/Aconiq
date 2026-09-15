import { act, renderHook } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type {
  ParameterDefinition,
  ProfileInfo,
  StandardDescriptor,
} from "@/api/client";
import { useRunSetupSelection } from "./use-run-setup-selection";

function profile(name: string, parameters: ParameterDefinition[]): ProfileInfo {
  return {
    name,
    supported_source_types: ["point"],
    supported_indicators: ["Lden"],
    parameters,
  };
}

function param(name: string, defaultValue?: string): ParameterDefinition {
  return {
    name,
    kind: "string",
    required: false,
    ...(defaultValue === undefined ? {} : { default_value: defaultValue }),
  };
}

const roadStandard: StandardDescriptor = {
  id: "rls19-road",
  description: "road",
  default_version: "2020",
  evidence_tier: "normative",
  versions: [
    {
      name: "2020",
      default_profile: "urban",
      profiles: [
        profile("urban", [param("grid_spacing", "10")]),
        // No default on the second parameter: seeding has to fall back to "".
        profile("rural", [param("grid_spacing", "25"), param("wind")]),
      ],
    },
    {
      name: "2019",
      default_profile: "legacy",
      profiles: [profile("legacy", [param("grid_spacing", "50")])],
    },
  ],
};

const railStandard: StandardDescriptor = {
  id: "schall03",
  description: "rail",
  default_version: "2014",
  evidence_tier: "normative",
  versions: [
    {
      name: "2014",
      default_profile: "standard",
      profiles: [profile("standard", [param("grid_spacing", "200")])],
    },
  ],
};

const standards = [roadStandard, railStandard];

describe("useRunSetupSelection", () => {
  it("resolves to the first standard and its declared defaults", () => {
    const { result } = renderHook(() => useRunSetupSelection(standards));

    expect(result.current.standardId).toBe("rls19-road");
    expect(result.current.version).toBe("2020");
    expect(result.current.profile).toBe("urban");
    expect(result.current.params).toEqual({ grid_spacing: "10" });
  });

  it("resolves to nothing while the standards list has not arrived", () => {
    const { result } = renderHook(() => useRunSetupSelection(undefined));

    expect(result.current.standardId).toBe("");
    expect(result.current.profileInfo).toBeUndefined();
    expect(result.current.params).toEqual({});
  });

  it("adopts the first standard when the list arrives late", () => {
    const { result, rerender } = renderHook(
      ({ list }: { list: StandardDescriptor[] | undefined }) =>
        useRunSetupSelection(list),
      { initialProps: { list: undefined as StandardDescriptor[] | undefined } },
    );

    expect(result.current.standardId).toBe("");
    rerender({ list: standards });
    expect(result.current.standardId).toBe("rls19-road");
    expect(result.current.params).toEqual({ grid_spacing: "10" });
  });

  it("resets version, profile and parameters when the standard changes", () => {
    const { result } = renderHook(() => useRunSetupSelection(standards));

    act(() => {
      result.current.selectStandard("schall03");
    });

    expect(result.current.version).toBe("2014");
    expect(result.current.profile).toBe("standard");
    expect(result.current.params).toEqual({ grid_spacing: "200" });
  });

  it("resets profile and parameters when the version changes", () => {
    const { result } = renderHook(() => useRunSetupSelection(standards));

    act(() => {
      result.current.selectVersion("2019");
    });

    expect(result.current.profile).toBe("legacy");
    expect(result.current.params).toEqual({ grid_spacing: "50" });
  });

  it("seeds a parameter with no declared default as an empty string", () => {
    const { result } = renderHook(() => useRunSetupSelection(standards));

    act(() => {
      result.current.selectProfile("rural");
    });

    expect(result.current.params).toEqual({ grid_spacing: "25", wind: "" });
  });

  it("drops the previous profile's fields rather than carrying them over", () => {
    const { result } = renderHook(() => useRunSetupSelection(standards));

    act(() => {
      result.current.selectProfile("rural");
    });
    act(() => {
      result.current.selectProfile("urban");
    });

    expect(result.current.params).toEqual({ grid_spacing: "10" });
    expect(result.current.params).not.toHaveProperty("wind");
  });

  it("returns a re-selected standard to its own defaults, not the last edit", () => {
    const { result } = renderHook(() => useRunSetupSelection(standards));

    act(() => {
      result.current.setParam("grid_spacing", "999");
    });
    expect(result.current.params).toEqual({ grid_spacing: "999" });

    act(() => {
      result.current.selectStandard("schall03");
    });
    act(() => {
      result.current.selectStandard("rls19-road");
    });

    expect(result.current.params).toEqual({ grid_spacing: "10" });
  });

  it("keeps an edited value until something above it changes", () => {
    const { result, rerender } = renderHook(() =>
      useRunSetupSelection(standards),
    );

    act(() => {
      result.current.setParam("grid_spacing", "42");
    });
    rerender();

    expect(result.current.params).toEqual({ grid_spacing: "42" });
  });
});

import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import type { ParameterDefinition } from "@/api/client";
import { ParameterField } from "./parameter-field";

function definition(
  overrides: Partial<ParameterDefinition> = {},
): ParameterDefinition {
  return {
    name: "grid_resolution_m",
    kind: "float",
    required: false,
    ...overrides,
  };
}

function renderField(
  param: ParameterDefinition,
  props: { standardId?: string; describedById?: string } = {},
) {
  render(
    <ParameterField
      standardId={props.standardId ?? "cnossos-road"}
      param={param}
      value="10"
      onChange={() => {}}
      {...(props.describedById === undefined
        ? {}
        : { describedById: props.describedById })}
    />,
  );
}

/**
 * `aria-describedby` may only name ids that are on the page. A field pointing
 * at an absent note is a description a screen reader announces as nothing,
 * which is worse than printing none — and a backend is free to publish a
 * parameter with no `description`, which is how the id came to be named
 * without the paragraph being rendered.
 */
describe("ParameterField", () => {
  it("names its note when there is one", () => {
    renderField(definition({ description: "Spacing of the receiver grid." }));
    const input = screen.getByRole("spinbutton");
    const described = input.getAttribute("aria-describedby");
    expect(described).toContain("param-grid_resolution_m-note");
    expect(
      document.getElementById("param-grid_resolution_m-note"),
    ).not.toBeNull();
  });

  it("names no note when the parameter has no description", () => {
    // `dummy-freefield` is uncatalogued, so nothing supplies a sentence the
    // backend left out.
    renderField(definition(), { standardId: "dummy-freefield" });
    const input = screen.getByRole("spinbutton");
    expect(input.getAttribute("aria-describedby")).toBeNull();
    expect(document.getElementById("param-grid_resolution_m-note")).toBeNull();
  });

  it("points at the group's note instead of printing its own", () => {
    renderField(definition({ description: "Spacing of the receiver grid." }), {
      describedById: "param-group-grid-note",
    });
    const input = screen.getByRole("spinbutton");
    expect(input.getAttribute("aria-describedby")).toBe(
      "param-group-grid-note",
    );
    expect(document.getElementById("param-grid_resolution_m-note")).toBeNull();
  });

  it("keeps the unit in the description chain", () => {
    renderField(definition({ unit: "m" }), { standardId: "dummy-freefield" });
    const input = screen.getByRole("spinbutton");
    expect(input.getAttribute("aria-describedby")).toBe(
      "param-grid_resolution_m-unit",
    );
  });
});

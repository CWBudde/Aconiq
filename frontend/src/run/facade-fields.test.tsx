import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import type { StandardDescriptor } from "@/api/client";
import {
  EMPTY_FACADE_DRAFT,
  facadeRunSpec,
  offersFacadeMode,
  type FacadeDraft,
} from "./facade-draft";
import { FacadeFields } from "./facade-fields";

function standard(rule?: string): StandardDescriptor {
  return {
    id: "rls19-road",
    description: "stub",
    default_version: "2019",
    versions: [],
    evidence_tier: "normative",
    ...(rule === undefined ? {} : { facade_placement_rule: rule }),
  };
}

/**
 * The mode is offered where the standard publishes a placement rule, and only
 * there: the run refuses every other standard, and a hand-kept list here would
 * be a second answer to which ones those are.
 */
describe("offersFacadeMode", () => {
  it("follows the descriptor's facade_placement_rule", () => {
    expect(offersFacadeMode(standard("rls19-facade"))).toBe(true);
    expect(offersFacadeMode(standard())).toBe(false);
    expect(offersFacadeMode(undefined)).toBe(false);
  });
});

/**
 * The draft becomes a spec only when the run could be asked for: at least one
 * building and a spacing above 0 m. There is no default spacing, as `aconiq
 * run` has none, so an empty field is an incomplete request, not 0 or 5.
 */
describe("facadeRunSpec", () => {
  const draft = (overrides: Partial<FacadeDraft>): FacadeDraft => ({
    ...EMPTY_FACADE_DRAFT,
    ...overrides,
  });

  it("reads one building per line, trimmed, skipping blank lines", () => {
    expect(
      facadeRunSpec(
        draft({
          buildings: "  way/1 \n\nMusterstraße 1\n",
          spacing: "2.5",
          includeCourtyard: true,
        }),
      ),
    ).toEqual({
      buildings: ["way/1", "Musterstraße 1"],
      spacingM: 2.5,
      includeCourtyard: true,
    });
  });

  it("is incomplete without a building or a positive spacing", () => {
    expect(facadeRunSpec(draft({ buildings: "", spacing: "5" }))).toBeNull();
    expect(facadeRunSpec(draft({ buildings: "a", spacing: "" }))).toBeNull();
    expect(facadeRunSpec(draft({ buildings: "a", spacing: "0" }))).toBeNull();
    expect(facadeRunSpec(draft({ buildings: "a", spacing: "-1" }))).toBeNull();
    expect(facadeRunSpec(draft({ buildings: "a", spacing: "x" }))).toBeNull();
  });
});

describe("FacadeFields", () => {
  it("edits the draft and offers the opt-in only for a normative standard", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <FacadeFields
        draft={EMPTY_FACADE_DRAFT}
        onChange={onChange}
        normative={true}
      />,
    );

    fireEvent.change(screen.getByLabelText(/Buildings/), {
      target: { value: "Musterstraße 1" },
    });
    expect(onChange).toHaveBeenLastCalledWith({
      ...EMPTY_FACADE_DRAFT,
      buildings: "Musterstraße 1",
    });

    fireEvent.change(screen.getByLabelText(/spacing/i), {
      target: { value: "4" },
    });
    expect(onChange).toHaveBeenLastCalledWith({
      ...EMPTY_FACADE_DRAFT,
      spacing: "4",
    });

    expect(screen.getByText(/Run over unconfirmed points/)).toBeTruthy();
    expect(screen.getByText(/Name at least one building/)).toBeTruthy();

    rerender(
      <FacadeFields
        draft={{ ...EMPTY_FACADE_DRAFT, buildings: "a", spacing: "4" }}
        onChange={onChange}
        normative={false}
      />,
    );
    expect(screen.queryByText(/Run over unconfirmed points/)).toBeNull();
    expect(screen.queryByText(/Name at least one building/)).toBeNull();
  });
});

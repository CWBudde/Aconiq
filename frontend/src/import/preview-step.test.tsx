/**
 * The preview's two finding callouts, and the plural they select.
 *
 * They are here because the counts used to be glued in front of a message that
 * said "validation error(s)": the sentence was assembled in JSX, so no message
 * ever saw the number and no plural rule could apply. The assertions below run
 * over both arms in both locales — a catalogue that drops one renders the
 * other, silently, and only a one-against-many comparison sees it.
 */

import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { PreviewStep } from "./preview-step";
import { m } from "@/i18n/messages";
import type {
  ModelFeature,
  ValidationIssue,
  ValidationReport,
} from "@/model/types";

function issue(featureId: string, level: "error" | "warning"): ValidationIssue {
  return {
    level,
    code: "building.height.required",
    featureId,
    params: {},
  };
}

function report(errors: number, warnings: number): ValidationReport {
  return {
    valid: errors === 0,
    errors: Array.from({ length: errors }, (_, i) =>
      issue(`e${String(i)}`, "error"),
    ),
    warnings: Array.from({ length: warnings }, (_, i) =>
      issue(`w${String(i)}`, "warning"),
    ),
    checkedAt: "2026-09-19T00:00:00Z",
  };
}

function renderPreview(findings: ValidationReport) {
  return render(
    <PreviewStep
      features={[]}
      receivers={[]}
      calcArea={null}
      skippedCount={0}
      report={findings}
      workspaceEmpty
      mergeSkips={{ features: 0, receivers: 0, calcArea: false }}
      onBack={() => undefined}
      onAdd={() => undefined}
      onReplace={() => undefined}
    />,
  );
}

describe("PreviewStep finding counts", () => {
  it("says one error in the singular", () => {
    renderPreview(report(1, 0));

    expect(
      screen.getByText(m.status_validation_errors({ count: 1 })),
    ).toBeInTheDocument();
  });

  it("says two errors in the plural", () => {
    renderPreview(report(2, 0));

    const heading = screen.getByText(m.status_validation_errors({ count: 2 }));
    expect(heading).toBeInTheDocument();
    // The two arms must actually differ in the base locale, or the assertion
    // above would hold for a message with no plural at all.
    expect(m.status_validation_errors({ count: 2 }, { locale: "en" })).not.toBe(
      m.status_validation_errors({ count: 1 }, { locale: "en" }),
    );
  });

  it("says one warning in the singular", () => {
    renderPreview(report(0, 1));

    expect(
      screen.getByText(m.msg_validation_warning_count({ count: 1 })),
    ).toBeInTheDocument();
  });

  it("says two warnings in the plural", () => {
    renderPreview(report(0, 2));

    expect(
      screen.getByText(m.msg_validation_warning_count({ count: 2 })),
    ).toBeInTheDocument();
  });

  it("carries no literal plural marker in either locale", () => {
    // The defect this file guards against, stated directly: a message that
    // spells both numbers at once rather than selecting between them.
    for (const locale of ["en", "de"] as const) {
      for (const count of [1, 2]) {
        expect(m.status_validation_errors({ count }, { locale })).not.toMatch(
          /\((s|e|en|n)\)/,
        );
        expect(
          m.msg_validation_warning_count({ count }, { locale }),
        ).not.toMatch(/\((s|e|en|n)\)/);
      }
    }
  });
});

describe("PreviewStep kind breakdown", () => {
  /*
   * Every kind the normalizer keeps needs a row here. The count above the list
   * comes off `features.length`, so a kind with no row of its own is imported
   * and paid for in that total while no line accounts for it — a zone-only
   * file then reads as one feature of nothing.
   */

  function groundZone(id: string): ModelFeature {
    return {
      id,
      kind: "ground-zone",
      properties: { ground_factor: 0.7 },
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 1],
            [0, 0],
          ],
        ],
      },
    };
  }

  it("counts ground zones under their own label", () => {
    render(
      <PreviewStep
        features={[groundZone("z1"), groundZone("z2")]}
        receivers={[]}
        calcArea={null}
        skippedCount={0}
        report={null}
        workspaceEmpty
        mergeSkips={{ features: 0, receivers: 0, calcArea: false }}
        onBack={() => undefined}
        onAdd={() => undefined}
        onReplace={() => undefined}
      />,
    );

    const label = screen.getByText(m.label_ground_zones());
    expect(label.parentElement?.textContent).toContain("2");
  });
});

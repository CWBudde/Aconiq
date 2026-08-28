import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { EVIDENCE_TIERS, type EvidenceTier } from "@/api/evidence-tier";
import { EvidenceTierBadge, EvidenceTierWarning } from "./evidence-tier-badge";
import { m } from "@/i18n/messages";

/**
 * The registry offers thirteen standards as peers when they are anything but.
 * These cover the whole tier vocabulary, plus the two ways the field can fail
 * to say anything useful: absent (an older backend) and unrecognised (a tier
 * added after this build). Neither may crash, and neither may render the word
 * "undefined" at the user.
 */

describe("EvidenceTierBadge", () => {
  it("names every known tier in words, not only in colour", () => {
    const expected: Record<EvidenceTier, string> = {
      normative: m.evidence_tier_normative(),
      preview: m.evidence_tier_preview(),
      scaffold: m.evidence_tier_scaffold(),
      "test-fixture": m.evidence_tier_test_fixture(),
    };

    for (const tier of EVIDENCE_TIERS) {
      const { unmount } = render(<EvidenceTierBadge tier={tier} />);

      const badge = screen.getByTestId("evidence-tier-badge");
      expect(badge).toHaveAttribute("data-tier", tier);
      expect(badge).toHaveTextContent(expected[tier]);

      unmount();
    }
  });

  it("gives every tier its own icon and border treatment", () => {
    const classes = new Map<string, string>();
    const icons = new Map<string, string>();

    for (const tier of EVIDENCE_TIERS) {
      const { unmount } = render(<EvidenceTierBadge tier={tier} />);
      const badge = screen.getByTestId("evidence-tier-badge");

      classes.set(tier, badge.className);

      const icon = badge.querySelector("svg");
      expect(icon).not.toBeNull();
      icons.set(tier, icon?.getAttribute("class") ?? "");

      unmount();
    }

    // Distinguishable without colour: the scaffold badge is the only one that
    // shouts, and the test fixture is the only one drawn with a dashed border.
    expect(classes.get("scaffold")).toContain("uppercase");
    expect(classes.get("normative")).not.toContain("uppercase");
    expect(classes.get("test-fixture")).toContain("border-dashed");
    expect(classes.get("normative")).not.toContain("border-dashed");

    // Each tier carries a distinct lucide icon, which survives greyscale.
    expect(new Set(icons.values()).size).toBe(EVIDENCE_TIERS.length);
  });

  it("labels the badge for screen readers", () => {
    render(<EvidenceTierBadge tier="scaffold" />);

    expect(screen.getByTestId("evidence-tier-badge")).toHaveTextContent(
      m.label_evidence_tier(),
    );
  });

  it("renders nothing when the backend sends no tier at all", () => {
    const { container } = render(<EvidenceTierBadge tier={undefined} />);

    expect(container).toBeEmptyDOMElement();
    expect(screen.queryByTestId("evidence-tier-badge")).toBeNull();
  });

  it("renders nothing for a blank tier", () => {
    const { container } = render(<EvidenceTierBadge tier="   " />);

    expect(container).toBeEmptyDOMElement();
  });

  it("falls back to a neutral badge for a tier it does not recognise", () => {
    render(<EvidenceTierBadge tier="quantum-normative" />);

    const badge = screen.getByTestId("evidence-tier-badge");
    expect(badge).toHaveAttribute("data-tier", "unknown");
    expect(badge).toHaveTextContent(m.evidence_tier_unknown());
    expect(badge.textContent).not.toContain("undefined");
    expect(badge.textContent).not.toContain("quantum-normative");
  });
});

describe("EvidenceTierWarning", () => {
  it("warns that a scaffold module is not usable for assessment", () => {
    render(<EvidenceTierWarning tier="scaffold" />);

    expect(screen.getByRole("alert")).toHaveTextContent(
      m.msg_evidence_tier_scaffold_warning(),
    );
  });

  it("stays silent for every other tier", () => {
    for (const tier of ["normative", "preview", "test-fixture"]) {
      const { container, unmount } = render(
        <EvidenceTierWarning tier={tier} />,
      );
      expect(container).toBeEmptyDOMElement();
      unmount();
    }
  });

  it("stays silent for a missing or unrecognised tier", () => {
    const { container: missing } = render(
      <EvidenceTierWarning tier={undefined} />,
    );
    expect(missing).toBeEmptyDOMElement();

    const { container: unknown } = render(<EvidenceTierWarning tier="later" />);
    expect(unknown).toBeEmptyDOMElement();
  });
});

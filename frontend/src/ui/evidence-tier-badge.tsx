import {
  Beaker,
  CircleQuestionMark,
  FlaskConical,
  ShieldCheck,
  TriangleAlert,
} from "lucide-react";
import {
  isScaffoldTier,
  parseEvidenceTier,
  type ResolvedEvidenceTier,
} from "@/api/evidence-tier";
import { m } from "@/i18n/messages";

/**
 * Evidence tier badges.
 *
 * Colour is never the only signal: every tier also carries its own word, its
 * own icon and its own border treatment, so the tier survives greyscale,
 * colour-blind vision and a high-contrast theme.
 *
 * | tier         | word         | icon      | border |
 * | ------------ | ------------ | --------- | ------ |
 * | normative    | Normative    | shield    | solid  |
 * | preview      | Preview      | flask     | solid  |
 * | scaffold     | Scaffold     | triangle  | solid, heavy, upper case |
 * | test-fixture | Test fixture | beaker    | dashed |
 * | unknown      | Unknown tier | question  | dotted |
 *
 * Labels are held as functions, not resolved strings: calling a message at
 * module scope freezes it to the locale active at import time.
 */
const tierConfig: Record<
  ResolvedEvidenceTier,
  {
    label: () => string;
    title: () => string;
    icon: React.ComponentType<{ className?: string }>;
    className: string;
  }
> = {
  normative: {
    label: m.evidence_tier_normative,
    title: m.evidence_tier_normative_desc,
    icon: ShieldCheck,
    // Unremarkable on purpose: normative is the baseline, not an award.
    className: "border-border bg-muted text-muted-foreground",
  },
  preview: {
    label: m.evidence_tier_preview,
    title: m.evidence_tier_preview_desc,
    icon: FlaskConical,
    className:
      "border-blue-300 bg-blue-50 text-blue-800 dark:border-blue-700 dark:bg-blue-950 dark:text-blue-200",
  },
  scaffold: {
    label: m.evidence_tier_scaffold,
    title: m.evidence_tier_scaffold_desc,
    icon: TriangleAlert,
    className:
      "border-amber-500 bg-amber-100 text-amber-900 font-semibold uppercase tracking-wide dark:border-amber-500 dark:bg-amber-950 dark:text-amber-100",
  },
  "test-fixture": {
    label: m.evidence_tier_test_fixture,
    title: m.evidence_tier_test_fixture_desc,
    icon: Beaker,
    className: "border-dashed border-border bg-muted/50 text-muted-foreground",
  },
  unknown: {
    label: m.evidence_tier_unknown,
    title: m.evidence_tier_unknown_desc,
    icon: CircleQuestionMark,
    className:
      "border-dotted border-border bg-transparent text-muted-foreground",
  },
};

/**
 * Renders the evidence tier of a standard.
 *
 * Renders nothing when the backend made no claim (an older backend that does
 * not send `evidence_tier` at all) — an absent claim is not a tier.
 */
export function EvidenceTierBadge({
  tier,
  className,
}: {
  tier: string | undefined;
  className?: string;
}) {
  const resolved = parseEvidenceTier(tier);
  if (resolved === null) return null;

  const cfg = tierConfig[resolved];
  const Icon = cfg.icon;

  return (
    <span
      data-testid="evidence-tier-badge"
      data-tier={resolved}
      title={cfg.title()}
      className={`inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium ${cfg.className} ${className ?? ""}`}
    >
      <Icon className="h-3 w-3 shrink-0" />
      <span className="sr-only">{m.label_evidence_tier()}: </span>
      {cfg.label()}
    </span>
  );
}

/**
 * The inline warning shown next to the run action when a scaffold-tier module
 * is selected. Renders nothing for every other tier — including an absent or
 * unrecognised one, which is not evidence of a scaffold.
 */
export function EvidenceTierWarning({ tier }: { tier: string | undefined }) {
  if (!isScaffoldTier(tier)) return null;

  return (
    <div
      role="alert"
      className="flex items-start gap-2 rounded-md border border-amber-400 bg-amber-50 p-3 text-xs text-amber-900 dark:border-amber-600 dark:bg-amber-950 dark:text-amber-100"
    >
      <TriangleAlert className="mt-0.5 h-3.5 w-3.5 shrink-0" />
      <span>{m.msg_evidence_tier_scaffold_warning()}</span>
    </div>
  );
}

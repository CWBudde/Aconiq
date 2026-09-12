import {
  Beaker,
  CircleQuestionMark,
  FlaskConical,
  ShieldCheck,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";
import {
  isScaffoldTier,
  parseEvidenceTier,
  type ResolvedEvidenceTier,
} from "@/api/evidence-tier";
import { m } from "@/i18n/messages";
import { Callout } from "@/ui/callout";
import { Badge, type BadgeProps } from "@/ui/components/badge";
import { cn } from "@/ui/lib/utils";

/**
 * Evidence tier badges.
 *
 * Colour is never the only signal: every tier also carries its own word, its
 * own icon and its own border treatment, so the tier survives greyscale,
 * colour-blind vision and a high-contrast theme.
 *
 * | tier         | word         | icon      | badge variant / border            |
 * | ------------ | ------------ | --------- | --------------------------------- |
 * | normative    | Normative    | shield    | secondary, solid                  |
 * | preview      | Preview      | flask     | info, solid                       |
 * | scaffold     | Scaffold     | triangle  | warning, solid, heavy, upper case |
 * | test-fixture | Test fixture | beaker    | outline, dashed                   |
 * | unknown      | Unknown tier | question  | outline, dotted                   |
 *
 * Labels are held as functions, not resolved strings: calling a message at
 * module scope freezes it to the locale active at import time.
 */
const tierConfig: Record<
  ResolvedEvidenceTier,
  {
    label: () => string;
    title: () => string;
    icon: LucideIcon;
    variant: NonNullable<BadgeProps["variant"]>;
    className?: string;
  }
> = {
  normative: {
    label: m.evidence_tier_normative,
    title: m.evidence_tier_normative_desc,
    icon: ShieldCheck,
    // Unremarkable on purpose: normative is the baseline, not an award.
    variant: "secondary",
  },
  preview: {
    label: m.evidence_tier_preview,
    title: m.evidence_tier_preview_desc,
    icon: FlaskConical,
    variant: "info",
  },
  scaffold: {
    label: m.evidence_tier_scaffold,
    title: m.evidence_tier_scaffold_desc,
    icon: TriangleAlert,
    variant: "warning",
    className: "font-semibold uppercase tracking-wide",
  },
  "test-fixture": {
    label: m.evidence_tier_test_fixture,
    title: m.evidence_tier_test_fixture_desc,
    icon: Beaker,
    variant: "outline",
    className: "border-dashed text-muted-foreground",
  },
  unknown: {
    label: m.evidence_tier_unknown,
    title: m.evidence_tier_unknown_desc,
    icon: CircleQuestionMark,
    variant: "outline",
    className: "border-dotted text-muted-foreground",
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
    <Badge
      variant={cfg.variant}
      data-testid="evidence-tier-badge"
      data-tier={resolved}
      title={cfg.title()}
      className={cn(cfg.className, className)}
    >
      <Icon aria-hidden="true" className="shrink-0" />
      <span className="sr-only">{m.label_evidence_tier()}: </span>
      {cfg.label()}
    </Badge>
  );
}

/**
 * The inline warning shown next to the run action when a scaffold-tier module
 * is selected. Renders nothing for every other tier — including an absent or
 * unrecognised one, which is not evidence of a scaffold.
 */
export function EvidenceTierWarning({ tier }: { tier: string | undefined }) {
  if (!isScaffoldTier(tier)) return null;

  // The warning variant of `Callout` carries `role="alert"` itself.
  return (
    <Callout variant="warning" icon={TriangleAlert}>
      {m.msg_evidence_tier_scaffold_warning()}
    </Callout>
  );
}

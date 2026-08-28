/**
 * Evidence tier — how far a standards module's output can be trusted.
 *
 * The registry offers its standards as peers, but they are not peers: some
 * carry real normative structure, coefficients and tables, while others are
 * scaffolds with invented numbers. The backend reports the difference as
 * `evidence_tier` on each standard descriptor; the UI must never let a
 * module's name assert more than its tier supports.
 */

/** The tiers this build knows how to describe. */
export const EVIDENCE_TIERS = [
  "normative",
  "preview",
  "scaffold",
  "test-fixture",
] as const;

export type EvidenceTier = (typeof EVIDENCE_TIERS)[number];

/**
 * A tier value that reached us but that this build cannot describe — a tier
 * added to the backend after this frontend was built. It is shown neutrally
 * rather than silently mapped onto a tier we do understand.
 */
export const UNKNOWN_EVIDENCE_TIER = "unknown";

export type ResolvedEvidenceTier = EvidenceTier | typeof UNKNOWN_EVIDENCE_TIER;

function isEvidenceTier(value: string): value is EvidenceTier {
  const known: readonly string[] = EVIDENCE_TIERS;
  return known.includes(value);
}

/**
 * Narrow a raw `evidence_tier` from the API.
 *
 * Returns `null` when the field is absent or blank — an older backend that
 * makes no claim at all, which is rendered as nothing rather than as a guess.
 * Returns `"unknown"` for a non-blank value this build does not recognise.
 */
export function parseEvidenceTier(
  raw: string | undefined,
): ResolvedEvidenceTier | null {
  if (raw === undefined) return null;
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  return isEvidenceTier(trimmed) ? trimmed : UNKNOWN_EVIDENCE_TIER;
}

/** True only for modules that carry no normative coefficients at all. */
export function isScaffoldTier(raw: string | undefined): boolean {
  return parseEvidenceTier(raw) === "scaffold";
}

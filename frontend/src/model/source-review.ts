import type { Geometry, ModelFeature } from "./types";

/**
 * A source's review state (P18 E2), in the shapes `modelgeojson` validates.
 *
 * A source traced from imagery is a proposal until a person confirms it;
 * which of them a run computes is the kernel's call (`sourcereview.Review`
 * through `reviewSources`), never this file's. What is here is what the
 * editor writes and what the run dialog needs to know before it asks: how
 * many sources are still proposed.
 */

export const SOURCE_REVIEW_STATUSES = [
  "proposed",
  "confirmed",
  "rejected",
] as const;

export type SourceReviewStatus = (typeof SOURCE_REVIEW_STATUSES)[number];

function isReviewStatus(value: unknown): value is SourceReviewStatus {
  return (
    typeof value === "string" &&
    (SOURCE_REVIEW_STATUSES as readonly string[]).includes(value)
  );
}

/**
 * A source's review status, or `undefined` for one nobody mapped from
 * imagery — taken from plans or an import — and for anything that is not a
 * source. A malformed value is the validator's to report, so it reads as none.
 */
export function sourceReviewStatus(
  feature: ModelFeature,
): SourceReviewStatus | undefined {
  if (feature.kind !== "source") return undefined;
  const value = feature.properties?.["review_status"];
  return isReviewStatus(value) ? value : undefined;
}

/** How many sources a normative run would refuse without the opt-in. */
export function proposedSourceCount(features: readonly ModelFeature[]): number {
  return features.filter(
    (feature) => sourceReviewStatus(feature) === "proposed",
  ).length;
}

/**
 * Whether a run lacks the opt-in it cannot run without: a normative standard
 * over proposed sources, which `aconiq run` refuses without
 * `--allow-unconfirmed`.
 */
export function sourceOptInMissing(
  proposed: number,
  normative: boolean,
  allowUnconfirmed: boolean,
): boolean {
  return normative && proposed > 0 && !allowUnconfirmed;
}

/**
 * The properties of a source just traced from imagery: proposed, with the
 * evidence it was read from.
 */
export function tracedFromImagery(
  properties: Record<string, unknown>,
  evidence: object,
): Record<string, unknown> {
  return { ...properties, review_status: "proposed", evidence: [evidence] };
}

/** The source with its status set; its evidence stays as it was. */
export function withSourceReviewStatus(
  feature: ModelFeature,
  status: SourceReviewStatus,
): ModelFeature {
  return {
    ...feature,
    properties: { ...feature.properties, review_status: status },
  };
}

/** A source's evidence entries, as stored; `[]` when it has none. */
export function sourceEvidence(
  feature: ModelFeature,
): Record<string, unknown>[] {
  const evidence = feature.properties?.["evidence"];
  if (!Array.isArray(evidence)) return [];
  return evidence.filter(
    (entry): entry is Record<string, unknown> =>
      typeof entry === "object" && entry !== null && !Array.isArray(entry),
  );
}

/**
 * A geometry's bounding box in its own coordinates, [minX, minY, maxX, maxY]:
 * the area the evidence image of a traced feature is requested for.
 */
export function featureBBox(
  geometry: Geometry,
): [number, number, number, number] {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  const walk = (node: unknown): void => {
    if (!Array.isArray(node)) return;
    const nodes = node as unknown[];
    const x = nodes[0];
    const y = nodes[1];
    if (typeof x === "number" && typeof y === "number") {
      minX = Math.min(minX, x);
      minY = Math.min(minY, y);
      maxX = Math.max(maxX, x);
      maxY = Math.max(maxY, y);
      return;
    }
    for (const child of nodes) walk(child);
  };
  walk(geometry.coordinates);
  return [minX, minY, maxX, maxY];
}

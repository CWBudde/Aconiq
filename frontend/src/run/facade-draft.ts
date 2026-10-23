import type { FacadeRunSpec } from "@/api/backend";
import type { StandardDescriptor } from "@/api/client";
import type { ModelReceiver } from "@/model/types";

/**
 * What the facade fields hold while the user types: the text as entered, so a
 * half-written spacing is not rounded away under the cursor. `facadeRunSpec`
 * turns it into what a run is asked for.
 */
export interface FacadeDraft {
  buildings: string;
  spacing: string;
  includeCourtyard: boolean;
  allowUnconfirmed: boolean;
}

export const EMPTY_FACADE_DRAFT: FacadeDraft = {
  buildings: "",
  spacing: "",
  includeCourtyard: false,
  allowUnconfirmed: false,
};

/**
 * Whether a standard has facade receivers at all. It reads the descriptor's
 * `facade_placement_rule`, which the Go side fills from the table the run
 * refuses by — the dialog keeps no list of its own.
 */
export function offersFacadeMode(
  standard: StandardDescriptor | undefined,
): boolean {
  return standard?.facade_placement_rule !== undefined;
}

/**
 * The facade set a run is asked for, or `null` while the draft is not one:
 * no building named, or no spacing above 0 m. One building per line, as an
 * id or "<street> <number>" — a line, because both may hold spaces and an
 * address may hold a comma. There is no default spacing, as `aconiq run` has
 * none: how densely a facade is sampled is the engineer's choice.
 */
export function facadeRunSpec(draft: FacadeDraft): FacadeRunSpec | null {
  const buildings = draft.buildings
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line !== "");
  const spacing = draft.spacing.trim() === "" ? NaN : Number(draft.spacing);
  if (buildings.length === 0 || !Number.isFinite(spacing) || spacing <= 0) {
    return null;
  }
  return {
    buildings,
    spacingM: spacing,
    includeCourtyard: draft.includeCourtyard,
  };
}

/**
 * The receiver properties `facadeset.AttributesOf` reads. A receiver carrying
 * any of them is facade-linked, and a normative custom run over it may be
 * refused as unconfirmed.
 */
const FACADE_REVIEW_PROPERTIES = [
  "placement_rule",
  "facade",
  "storey",
  "slab_height_source",
  "review_status",
] as const;

/**
 * Whether a custom run over these receivers could need the unconfirmed-point
 * opt-in. It only decides whether the dialog offers the checkbox; whether the
 * run is refused is the kernel's or the server's call, as it is for a facade
 * run.
 */
export function receiversCarryFacadeReview(
  receivers: readonly ModelReceiver[],
): boolean {
  return receivers.some((receiver) =>
    FACADE_REVIEW_PROPERTIES.some(
      (key) => receiver.properties !== undefined && key in receiver.properties,
    ),
  );
}

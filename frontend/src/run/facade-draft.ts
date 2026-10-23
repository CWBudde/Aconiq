import type { FacadeRunSpec } from "@/api/backend";
import type { StandardDescriptor } from "@/api/client";

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
 * Whether a facade run lacks the opt-in it cannot run without. Every point the
 * generator places is `proposed`, and a normative run refuses proposed points
 * unless it opts in — so without the tick the run, and its estimate, could
 * only come back refused.
 */
export function facadeOptInMissing(
  draft: FacadeDraft,
  normative: boolean,
): boolean {
  return normative && !draft.allowUnconfirmed;
}

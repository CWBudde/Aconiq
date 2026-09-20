import { useMemo } from "react";
import { useModelStore } from "./model-store";
import { validateProjectModel } from "./validate";
import type { ModelFeature, ModelReceiver, ValidationReport } from "./types";

export type ModelValidationState = "empty" | "valid" | "issues";

export interface ModelValidationSummary {
  state: ModelValidationState;
  /** 0 when the model is empty — an empty model has nothing to report yet. */
  errorCount: number;
  warningCount: number;
  /** null when the model is empty; nothing was validated. */
  report: ValidationReport | null;
}

/**
 * The one place the workspace is validated.
 *
 * Two things it fixes over calling the validator at each site.
 *
 * It passes the **receivers**. The former `validateModel` hardcoded `[]` for
 * them, so a model whose only defect is a receiver read as valid — in the map
 * panel, in its badge, and soon in the run gate, all of which had their own
 * copy of the call. That entry point is gone; this hook is how the store is
 * validated.
 *
 * And it answers "empty" with an early return **above** the validator rather
 * than by filtering the result. `validateProjectModel` pushes a synthetic
 * `model.empty` error, so a fresh install would otherwise be greeted with
 * "1 error" for having nothing in it yet. Filtering it out afterwards is the
 * tempting alternative and is worse: the displayed count would then disagree
 * with `report.valid`, which the run gate reads from the same report.
 */
export function useModelValidation(): ModelValidationSummary {
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);

  return useMemo(
    () => validationFor(features, receivers),
    [features, receivers],
  );
}

/**
 * The process-wide memo behind the hook.
 *
 * `useMemo` is per component instance, and this hook now has four callers in
 * one commit — the panel, the toggle badge that is always mounted, the docked
 * editor's per-feature list, and the workspace's finding queue. On the project
 * this was built for that is four full validations of 608 features on every
 * property edit, every undo and every geometry commit.
 *
 * Reference equality is the right key and not an approximation: every setter in
 * `model-store.ts` replaces the array rather than mutating it — `[...s.features,
 * f]`, `.filter`, `.map`, and both undo paths splice a `[...copy]` — so two
 * callers see the same array object exactly when they are looking at the same
 * model.
 *
 * A single entry, so at most one superseded model is pinned. The returned
 * report is now shared by reference, which callers depend on (a `useMemo` over
 * `report` is stable across renders) and which they must therefore not mutate.
 */
let cached: {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  value: ModelValidationSummary;
} | null = null;

/**
 * The same summary, for a caller that is handling an event rather than
 * rendering.
 *
 * An event handler that has just written to the store holds a `report` from the
 * render *before* the write, and deriving anything from it — the finding queue,
 * above all — answers the question the reader asked one edit ago. Reading the
 * fresh store and asking here is what makes "accept this and move on" land on
 * the finding that took the accepted one's place rather than wherever the old
 * queue pointed.
 *
 * It costs no extra validation: the cache below is keyed on the two array
 * identities, so the render that follows the write finds this result waiting
 * instead of computing its own. The work is moved earlier, not doubled.
 */
export function modelValidation(
  features: ModelFeature[],
  receivers: ModelReceiver[],
): ModelValidationSummary {
  return validationFor(features, receivers);
}

function validationFor(
  features: ModelFeature[],
  receivers: ModelReceiver[],
): ModelValidationSummary {
  if (
    cached &&
    cached.features === features &&
    cached.receivers === receivers
  ) {
    return cached.value;
  }

  const value = computeValidation(features, receivers);
  cached = { features, receivers, value };
  return value;
}

function computeValidation(
  features: ModelFeature[],
  receivers: ModelReceiver[],
): ModelValidationSummary {
  if (features.length === 0 && receivers.length === 0) {
    return { state: "empty", errorCount: 0, warningCount: 0, report: null };
  }
  const report = validateProjectModel(features, receivers);
  return {
    state:
      report.errors.length + report.warnings.length === 0 ? "valid" : "issues",
    errorCount: report.errors.length,
    warningCount: report.warnings.length,
    report,
  };
}

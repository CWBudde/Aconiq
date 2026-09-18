import { useMemo } from "react";
import { useModelStore } from "./model-store";
import { validateProjectModel } from "./validate";
import type { ValidationReport } from "./types";

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

  return useMemo(() => {
    if (features.length === 0 && receivers.length === 0) {
      return { state: "empty", errorCount: 0, warningCount: 0, report: null };
    }
    const report = validateProjectModel(features, receivers);
    return {
      state:
        report.errors.length + report.warnings.length === 0
          ? "valid"
          : "issues",
      errorCount: report.errors.length,
      warningCount: report.warnings.length,
      report,
    };
  }, [features, receivers]);
}

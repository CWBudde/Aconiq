import { useCallback } from "react";
import { useStandards } from "@/api/hooks";
import { getStandardLabel } from "@/run/standards-meta";

/**
 * Name a standard from a run, tier and all.
 *
 * `RunSummary` carries `standard_id` and no `evidence_tier`, so the run list,
 * the filter bar and the run detail header cannot qualify a name from what
 * they hold. They look it up here instead of the frontend keeping its own
 * opinion about which modules are scaffolds — see `standards-meta.ts` for why
 * that opinion had to go.
 *
 * The standards query is shared and long-lived (`staleTime` five minutes), so
 * this is one fetch per session rather than one per call site. Until it
 * resolves the name comes back unqualified, which is what an unanswered
 * question looks like; `EvidenceTierBadge` renders an absent tier the same way.
 */
export function useStandardLabel(): (standardId: string) => string {
  const { data } = useStandards();

  return useCallback(
    (standardId: string) =>
      getStandardLabel(
        standardId,
        data?.find((s) => s.id === standardId)?.evidence_tier,
      ),
    [data],
  );
}

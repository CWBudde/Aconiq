import { useIsMutating, useMutation, useQuery } from "@tanstack/react-query";
import { backend } from "./backend";
import type { OsmImportRequest, RunSpec } from "./backend";
import type { ModelSaveRequest, RasterMetadata, ReceiverTable } from "./client";
import { queryKeys } from "./query-keys";
import { queryClient } from "./query-client";

export function useHealth() {
  return useQuery({
    queryKey: queryKeys.health.all,
    queryFn: () => backend.getHealth(),
    staleTime: 60_000,
  });
}

export function useProjectStatus() {
  return useQuery({
    queryKey: queryKeys.project.status(),
    queryFn: () => backend.getProjectStatus(),
  });
}

export function useStandards() {
  return useQuery({
    queryKey: queryKeys.standards.all,
    queryFn: () => backend.getStandards(),
    staleTime: 5 * 60_000,
  });
}

export function useRuns(refetchIntervalMs?: number) {
  const refetchInterval = backend.capabilities.runsChangeExternally
    ? refetchIntervalMs
    : undefined;
  return useQuery({
    queryKey: queryKeys.runs.list(),
    queryFn: () => backend.getRuns(),
    ...(refetchInterval === undefined ? {} : { refetchInterval }),
  });
}

export function useRunLog(runId: string | null) {
  return useQuery({
    queryKey: queryKeys.runs.log(runId ?? ""),
    queryFn: () => {
      if (!runId) throw new Error("Run ID is required");
      return backend.getRunLog(runId);
    },
    enabled: runId !== null,
    staleTime: 30_000,
  });
}

export function useArtifactContent<T>(artifactId: string | null) {
  return useQuery({
    queryKey: queryKeys.artifacts.content(artifactId ?? ""),
    queryFn: () => {
      if (!artifactId) throw new Error("Artifact ID is required");
      return backend.getArtifactContent<T>(artifactId);
    },
    enabled: artifactId !== null,
    staleTime: 5 * 60_000,
  });
}

export function useReceiverTable(artifactId: string | null) {
  return useArtifactContent<ReceiverTable>(artifactId);
}

export function useRasterMetadata(artifactId: string | null) {
  return useArtifactContent<RasterMetadata>(artifactId);
}

export function useImportFromOSM() {
  return useMutation({
    mutationFn: (req: OsmImportRequest) => backend.importFromOSM(req),
  });
}

export function useCreateRun() {
  return useMutation({
    // A refusal is thrown whole: the run dialog reads `code` and `hint` off
    // the envelope to explain one the user can act on.
    mutationFn: (spec: RunSpec) => backend.startRun(spec),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.runs.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.project.all });
    },
  });
}

export function useCreateExport() {
  return useMutation({
    mutationFn: (runId: string) => backend.createExport(runId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.runs.all });
    },
  });
}

/**
 * Keyed so that "is a save in flight?" can be asked from anywhere
 * (`useIsSavingModel`), not only by the component that started it.
 */
export const MODEL_SAVE_MUTATION_KEY = ["model", "save"] as const;

export function useSaveModel() {
  return useMutation({
    mutationKey: MODEL_SAVE_MUTATION_KEY,
    mutationFn: (req: ModelSaveRequest) => backend.saveModel(req),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.project.all });
    },
  });
}

/** True while any `useSaveModel` mutation is running, whichever surface started it. */
export function useIsSavingModel(): boolean {
  return (
    useIsMutating({ mutationKey: MODEL_SAVE_MUTATION_KEY }, queryClient) > 0
  );
}

export function getArtifactContentURL(artifactId: string): string {
  return backend.getArtifactURL(artifactId);
}

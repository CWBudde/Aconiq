import { useMutation, useQuery } from "@tanstack/react-query";
import { browserBackend, type BrowserRunSpec } from "./browser-backend";
import { IS_WASM_MODE, apiURL } from "./mode";
import { queryKeys } from "./query-keys";
import { queryClient } from "./query-client";
import type {
  CreateRunRequest,
  HealthResponse,
  ProjectStatusResponse,
  RasterMetadata,
  ReceiverTable,
  RunLog,
  RunSummary,
  StandardDescriptor,
} from "./client";
import type { GeoJSONFeatureCollection } from "@/model/types";

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(apiURL(path), {
    headers: { Accept: "application/json" },
  });

  if (!response.ok) {
    throw new Error(`Request failed: ${String(response.status)}`);
  }

  return (await response.json()) as T;
}

/** Ensure the WASM kernel is loaded (registers window.aconiq). */
async function ensureKernel(): Promise<void> {
  const { getKernel } = await import("@/wasm/kernel");
  await getKernel();
}

export function useHealth() {
  return useQuery({
    queryKey: queryKeys.health.all,
    queryFn: async () => {
      if (IS_WASM_MODE) {
        return browserBackend.getHealth();
      }
      return fetchJSON<HealthResponse>("/api/v1/health");
    },
    staleTime: 60_000,
  });
}

export function useProjectStatus() {
  return useQuery({
    queryKey: queryKeys.project.status(),
    queryFn: async () => {
      if (IS_WASM_MODE) {
        return browserBackend.getProjectStatus();
      }
      const response = await fetch(apiURL("/api/v1/project/status"), {
        headers: { Accept: "application/json" },
      });

      if (response.status === 404) {
        return null;
      }
      if (!response.ok) {
        throw new Error(`Request failed: ${String(response.status)}`);
      }
      return (await response.json()) as ProjectStatusResponse;
    },
  });
}

export function useStandards() {
  return useQuery({
    queryKey: queryKeys.standards.all,
    queryFn: () =>
      IS_WASM_MODE
        ? browserBackend.getStandards()
        : fetchJSON<StandardDescriptor[]>("/api/v1/standards"),
    staleTime: 5 * 60_000,
  });
}

export function useRuns(refetchIntervalMs?: number) {
  return useQuery({
    queryKey: queryKeys.runs.list(),
    queryFn: () =>
      IS_WASM_MODE
        ? browserBackend.getRuns()
        : fetchJSON<RunSummary[]>("/api/v1/runs"),
    ...(IS_WASM_MODE
      ? {}
      : refetchIntervalMs !== undefined
        ? { refetchInterval: refetchIntervalMs }
        : {}),
  });
}

export function useRunLog(runId: string | null) {
  return useQuery({
    queryKey: queryKeys.runs.log(runId ?? ""),
    queryFn: () => {
      if (!runId) throw new Error("Run ID is required");
      return IS_WASM_MODE
        ? browserBackend.getRunLog(runId)
        : fetchJSON<RunLog>(`/api/v1/runs/${runId}/log`);
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
      return IS_WASM_MODE
        ? browserBackend.getArtifactContent<T>(artifactId)
        : fetchJSON<T>(`/api/v1/artifacts/${artifactId}/content`);
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

export interface OsmImportRequest {
  south: number;
  west: number;
  north: number;
  east: number;
  overpass_endpoint?: string;
}

export function useImportFromOSM() {
  return useMutation({
    mutationFn: async (req: OsmImportRequest) => {
      if (IS_WASM_MODE) {
        return browserBackend.importFromOSM(req);
      }
      const response = await fetch(apiURL("/api/v1/import/osm"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify(req),
      });
      if (!response.ok) {
        const payload = (await response.json().catch(() => null)) as {
          error?: { message?: string };
        } | null;
        throw new Error(
          payload?.error?.message ??
            `Request failed: ${String(response.status)}`,
        );
      }
      return response.json() as Promise<GeoJSONFeatureCollection>;
    },
  });
}

export function useCreateRun() {
  return useMutation({
    mutationFn: async (spec: BrowserRunSpec) => {
      if (IS_WASM_MODE) {
        await ensureKernel();
        return browserBackend.startRun(spec);
      }

      const request: CreateRunRequest = {
        standard_id: spec.standardId,
        standard_version: spec.version,
        standard_profile: spec.profile,
        receiver_mode: spec.receiverMode,
        params: spec.params,
      };

      const response = await fetch(apiURL("/api/v1/runs"), {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify(request),
      });
      if (!response.ok) {
        const payload = (await response.json().catch(() => null)) as {
          error?: { message?: string };
        } | null;
        throw new Error(
          payload?.error?.message ??
            `Request failed: ${String(response.status)}`,
        );
      }
      return response.json() as Promise<RunSummary>;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.runs.all });
      await queryClient.invalidateQueries({ queryKey: queryKeys.project.all });
    },
  });
}

export function useCreateExport() {
  return useMutation({
    mutationFn: async (runId: string) => {
      if (!IS_WASM_MODE) {
        throw new Error(
          "Export generation from the UI is only available in browser mode",
        );
      }
      return browserBackend.createExport(runId);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: queryKeys.runs.all });
    },
  });
}

export function getArtifactContentURL(artifactId: string): string {
  return IS_WASM_MODE
    ? browserBackend.getArtifactURL(artifactId)
    : apiURL(`/api/v1/artifacts/${artifactId}/content`);
}

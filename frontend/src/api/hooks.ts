import { useMutation, useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import type {
  HealthResponse,
  ProjectStatusResponse,
  RasterMetadata,
  ReceiverTable,
  RunLog,
  RunSummary,
  StandardDescriptor,
} from "./client";
import type { GeoJSONFeatureCollection } from "@/model/types";

const API_BASE = "";

async function fetchJSON<T>(path: string): Promise<T> {
  const response = await fetch(`${API_BASE}${path}`, {
    headers: { Accept: "application/json" },
  });

  if (!response.ok) {
    throw new Error(`Request failed: ${String(response.status)}`);
  }

  return (await response.json()) as T;
}

export function useHealth() {
  return useQuery({
    queryKey: queryKeys.health.all,
    queryFn: () => fetchJSON<HealthResponse>("/api/v1/health"),
    staleTime: 60_000,
  });
}

export function useProjectStatus() {
  return useQuery({
    queryKey: queryKeys.project.status(),
    queryFn: () => fetchJSON<ProjectStatusResponse>("/api/v1/project/status"),
  });
}

export function useStandards() {
  return useQuery({
    queryKey: queryKeys.standards.all,
    queryFn: () => fetchJSON<StandardDescriptor[]>("/api/v1/standards"),
    staleTime: 5 * 60_000,
  });
}

export function useRuns(refetchIntervalMs?: number) {
  return useQuery({
    queryKey: queryKeys.runs.list(),
    queryFn: () => fetchJSON<RunSummary[]>("/api/v1/runs"),
    refetchInterval: refetchIntervalMs,
  });
}

export function useRunLog(runId: string | null) {
  return useQuery({
    queryKey: queryKeys.runs.log(runId ?? ""),
    queryFn: () => fetchJSON<RunLog>(`/api/v1/runs/${runId}/log`),
    enabled: runId !== null,
    staleTime: 30_000,
  });
}

export function useArtifactContent<T>(artifactId: string | null) {
  return useQuery({
    queryKey: queryKeys.artifacts.content(artifactId ?? ""),
    queryFn: () => fetchJSON<T>(`/api/v1/artifacts/${artifactId}/content`),
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
      const response = await fetch("/api/v1/import/osm", {
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

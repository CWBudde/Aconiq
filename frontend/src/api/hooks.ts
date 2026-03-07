import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import type {
  HealthResponse,
  ProjectStatusResponse,
  RunLog,
  RunSummary,
  StandardDescriptor,
} from "./client";

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

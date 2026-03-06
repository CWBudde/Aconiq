import { useQuery } from "@tanstack/react-query";
import { queryKeys } from "./query-keys";
import type { HealthResponse, ProjectStatusResponse } from "./client";

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

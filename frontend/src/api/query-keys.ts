/**
 * Query key factory — central registry for all TanStack Query keys.
 *
 * Convention: each domain gets a top-level key array, with sub-keys
 * for specific queries. This makes targeted invalidation easy:
 *   queryClient.invalidateQueries({ queryKey: queryKeys.project.all })
 */
export const queryKeys = {
  health: {
    all: ["health"] as const,
  },
  project: {
    all: ["project"] as const,
    status: () => [...queryKeys.project.all, "status"] as const,
  },
  standards: {
    all: ["standards"] as const,
  },
  runs: {
    all: ["runs"] as const,
    list: () => [...queryKeys.runs.all, "list"] as const,
    log: (id: string) => [...queryKeys.runs.all, id, "log"] as const,
  },
} as const;

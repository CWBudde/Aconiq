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
} as const;

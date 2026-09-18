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
  artifacts: {
    all: ["artifacts"] as const,
    content: (id: string) =>
      [...queryKeys.artifacts.all, id, "content"] as const,
    // Keyed apart from `content`, not alongside it: the bytes of one artifact
    // are megabytes, and sharing an entry with the parsed JSON would mean a
    // metadata read and a raster read overwriting each other's cached value.
    bytes: (id: string) => [...queryKeys.artifacts.all, id, "bytes"] as const,
  },
} as const;

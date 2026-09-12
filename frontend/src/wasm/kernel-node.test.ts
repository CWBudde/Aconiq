// Health check for the Node-side kernel loader.
//
// It is deliberately separate from kernel-parity.test.ts: when the loader
// breaks, this is one obvious failure rather than thirty-six confusing ones.

import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { getNodeKernel, kernelSkipReason } from "./kernel-node";

const skipReason = kernelSkipReason();

/** Read a file from the repository, relative to this test's directory. */
function repoFile(relative: string): string {
  return readFileSync(
    fileURLToPath(new URL(relative, import.meta.url)),
    "utf8",
  );
}

/**
 * The scalars `road.DefaultPropagationConfig` returns, parsed out of the Go
 * source. The kernel's `defaultConfig()` is the browser's only view of them, so
 * a change on the Go side that never reaches the WASM build shows up here.
 */
function goDefaultConfig(): Record<string, number> {
  const source = repoFile(
    "../../../backend/internal/standards/rls19/road/propagation.go",
  );

  const body = /func DefaultPropagationConfig\(\)[^{]*\{[^}]*\{([^}]*)\}/.exec(
    source,
  );
  const fields = body?.[1];
  if (fields === undefined) {
    throw new Error("DefaultPropagationConfig not found in propagation.go");
  }

  const defaults: Record<string, number> = {};
  for (const match of fields.matchAll(/(\w+):\s*([0-9]+(?:\.[0-9]+)?)\s*,/g)) {
    const [, name, value] = match;
    if (name !== undefined && value !== undefined) {
      defaults[name] = Number(value);
    }
  }

  return defaults;
}

describe.skipIf(skipReason !== null)("Node WASM kernel loader", () => {
  it("returns the same defaults the Go source declares", async () => {
    const kernel = await getNodeKernel();
    const defaults = kernel.defaultConfig();
    const want = goDefaultConfig();

    expect(Object.keys(want).length).toBeGreaterThan(0);
    for (const [name, value] of Object.entries(want)) {
      expect(defaults[name as keyof typeof defaults]).toBe(value);
    }
  });

  it("computes a level for a well-formed request", async () => {
    const kernel = await getNodeKernel();

    const outputs = await kernel.rls19Road({
      receivers: [{ id: "r1", point: { x: 0, y: 50 }, height_m: 4 }],
      sources: [
        {
          id: "road",
          centerline: [
            { x: -50, y: 0 },
            { x: 50, y: 0 },
          ],
          surface_type: "SMA",
          speeds: {
            pkw_kph: 100,
            lkw1_kph: 80,
            lkw2_kph: 70,
            krad_kph: 100,
          },
          traffic_day: {
            pkw_per_hour: 900,
            lkw1_per_hour: 40,
            lkw2_per_hour: 60,
            krad_per_hour: 10,
          },
          traffic_night: {
            pkw_per_hour: 200,
            lkw1_per_hour: 10,
            lkw2_per_hour: 20,
            krad_per_hour: 2,
          },
        },
      ],
      barriers: [],
      config: { SegmentLengthM: 5, MinDistanceM: 3, ReceiverHeightM: 4 },
    });

    expect(outputs).toHaveLength(1);
    expect(outputs[0]?.Indicators.lr_day).toBeGreaterThan(
      outputs[0]?.Indicators.lr_night ?? 0,
    );
  });

  it("rejects a malformed request rather than answering it", async () => {
    const kernel = await getNodeKernel();

    // A source with no centerline cannot be validated into a scene; the kernel
    // must say so instead of returning a level for it.
    await expect(
      kernel.rls19Road({
        receivers: [{ id: "r1", point: { x: 0, y: 50 }, height_m: 4 }],
        sources: [{ id: "bad" } as never],
        barriers: [],
        config: { SegmentLengthM: 1, MinDistanceM: 3, ReceiverHeightM: 4 },
      }),
    ).rejects.toBeTruthy();
  });
});

if (skipReason !== null) {
  describe("Node WASM kernel loader", () => {
    it.skip(`skipped: ${skipReason}`, () => {
      // Recorded so the skip carries its reason into the vitest output.
    });
  });
}

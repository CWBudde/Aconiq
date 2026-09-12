// Browser-CLI parity, layer 1: does the WASM build of the kernel compute the
// same numbers the native Go build does?
//
// Every other frontend test that touches the kernel pins *names* — the surface
// vocabulary, the Parkplatz vocabulary, the descriptor's parameter enum. None of
// them can see a wrong level. This one drives the real `GOOS=js GOARCH=wasm`
// artifact over the whole CI-safe acceptance suite and compares dB against the
// goldens the Go tree owns, at the suite's own 1e-6 dB tolerance.
//
// It is layer 1 of two on purpose. browser-parity.test.ts checks that
// browser-backend.ts builds the same *scene* from a model that the CLI does;
// without this test a failure there could equally mean the kernel itself
// diverged, and the two would be indistinguishable.

import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

import { getNodeKernel, kernelSkipReason } from "./kernel-node";
import type {
  Barrier,
  Building,
  ComputeRequest,
  ParkingSource,
  PointReceiver,
  PropagationConfig,
  Reflector,
  RoadSource,
  TerrainProfile,
} from "./types";

// Resolved through node:path rather than `new URL(rel, import.meta.url)`: Vite
// rewrites that idiom at transform time into a served `/@fs/...` URL, which
// fileURLToPath then refuses. Decoding import.meta.url once and joining with
// path.resolve is not rewritten and gives the real file on disk.
const SUITE_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  "../../../backend/internal/qa/acceptance/rls19_test20/testdata",
);

const skipReason = kernelSkipReason();

function suiteFile(relative: string): string {
  return readFileSync(resolve(SUITE_DIR, relative), "utf8");
}

/** One entry of ci_safe_suite.json. */
interface SuiteTask {
  name: string;
  category: string;
  scenario_path: string;
  expected_path?: string;
  tolerance: { absolute_db: number };
}

/**
 * The acceptance runner's own scenario DTO (`scenarioFile` in runner.go). It is
 * deliberately not the kernel's request shape — `buildings` sits at the top
 * level and the config members are snake_case — so the translation below has to
 * mirror `propagationConfigFile.toPropagationConfig` exactly.
 */
interface ScenarioFile {
  sources: RoadSource[];
  barriers?: Barrier[];
  buildings?: Building[];
  receivers: PointReceiver[];
  propagation_config: {
    segment_length_m: number;
    min_distance_m: number;
    receiver_height_m: number;
    receiver_terrain_z?: number;
    terrain?: TerrainProfile[];
    reflectors?: Reflector[];
    parking_sources?: ParkingSource[];
  };
}

interface ReceiverSnapshot {
  id: string;
  x: number;
  y: number;
  height_m: number;
  lr_day: number;
  lr_night: number;
}

/** runner.go:515 — the goldens are written through this, so comparisons are too. */
function round6(value: number): number {
  return Math.round(value * 1e6) / 1e6;
}

/**
 * runner.go:391 in TypeScript. Buildings move from the scenario's top level into
 * the config, because that is where the Go module carries them; everything else
 * is a snake_case → PascalCase rename, since `road.PropagationConfig` has no
 * json tags while every type it contains does.
 */
function toComputeRequest(scenario: ScenarioFile): ComputeRequest {
  const source = scenario.propagation_config;

  const config: PropagationConfig = {
    SegmentLengthM: source.segment_length_m,
    MinDistanceM: source.min_distance_m,
    ReceiverHeightM: source.receiver_height_m,
  };

  // Assigned conditionally rather than spread with undefined: the project runs
  // with exactOptionalPropertyTypes, and an explicit `Terrain: undefined` is
  // not the same type as an absent key.
  if (source.receiver_terrain_z !== undefined) {
    config.ReceiverTerrainZ = source.receiver_terrain_z;
  }
  if (source.terrain !== undefined) {
    config.Terrain = source.terrain;
  }
  if (source.reflectors !== undefined) {
    config.Reflectors = source.reflectors;
  }
  if (scenario.buildings !== undefined) {
    config.Buildings = scenario.buildings;
  }
  if (source.parking_sources !== undefined) {
    config.ParkingSources = source.parking_sources;
  }

  return {
    receivers: scenario.receivers,
    sources: scenario.sources,
    barriers: scenario.barriers ?? [],
    config,
  };
}

const suite = JSON.parse(suiteFile("ci_safe_suite.json")) as {
  tasks: SuiteTask[];
};

// Every task with a golden. Reading the manifest rather than globbing means a
// fixture added on the Go side is picked up here without anyone remembering to.
const tasks = suite.tasks.filter((task) => task.expected_path !== undefined);

describe.skipIf(skipReason !== null)("WASM kernel vs. CI-safe goldens", () => {
  it("covers every task the acceptance manifest declares", () => {
    expect(tasks.length).toBe(suite.tasks.length);
    expect(tasks.length).toBeGreaterThan(30);
  });

  it.each(tasks.map((task) => [task.name, task] as const))(
    "%s",
    async (_name, task) => {
      const scenario = JSON.parse(
        suiteFile(task.scenario_path),
      ) as ScenarioFile;
      const expected = JSON.parse(suiteFile(task.expected_path as string)) as {
        receivers: ReceiverSnapshot[];
      };

      const kernel = await getNodeKernel();
      const outputs = await kernel.rls19Road(toComputeRequest(scenario));

      expect(outputs).toHaveLength(expected.receivers.length);

      const tolerance = task.tolerance.absolute_db;
      outputs.forEach((output, index) => {
        const want = expected.receivers[index];
        expect(want).toBeDefined();
        expect(output.Receiver.id).toBe(want?.id);

        // round6 before comparing, exactly as the Go runner does: the goldens
        // are written rounded, so an unrounded comparison would spend half the
        // 1e-6 dB budget on the rounding itself.
        expect(
          Math.abs(round6(output.Indicators.lr_day) - (want?.lr_day ?? NaN)),
        ).toBeLessThanOrEqual(tolerance);
        expect(
          Math.abs(
            round6(output.Indicators.lr_night) - (want?.lr_night ?? NaN),
          ),
        ).toBeLessThanOrEqual(tolerance);
      });
    },
  );
});

if (skipReason !== null) {
  describe("WASM kernel vs. CI-safe goldens", () => {
    it.skip(`skipped: ${skipReason}`, () => {
      // Recorded so the skip carries its reason into the runner output.
    });
  });
}

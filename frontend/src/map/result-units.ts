/**
 * Whether a result container says its values are decibels at all.
 *
 * Its own module because two containers now have to answer it the same way:
 * the receiver table `ResultLayers` paints as circles, and the raster sidecar
 * `useResultRaster` paints as a surface. One function, one comment — a copy
 * would let the two gates drift, and a raster painted under a gate the circles
 * beside it did not apply is exactly the disagreement neither would report.
 */

/**
 * `NOISE_LEVEL_RAMP` is a decibel ramp with fixed 35–80 dB stops, so it may
 * only paint a container that claims to hold levels. Every level-producing
 * module writes `"dB"` (`"dB(A)"` in browser mode); `beb-exposure` writes
 * `"mixed"`, because its `indicator_order` puts Lden and Lnight beside
 * dwelling and person *counts* (`standards/beb/exposure/export.go`). Running
 * one of those counts through the ramp would present a population total as an
 * acoustic level, under a legend that still reads in dB.
 *
 * Decided on the unit rather than on a list of indicator names: the names are
 * the backend's and would have to be copied here and kept in step, while the
 * unit is a field both result containers already carry.
 */
export function declaresLevels(unit: string): boolean {
  return unit.trim().toLowerCase().startsWith("db");
}

/**
 * The unit a named channel's values carry, or `""` when the container names
 * none.
 *
 * `""` rather than a thrown error or a guessed default: a container written
 * before the unit was per channel expands its old scalar across every channel
 * on read, so a genuinely absent unit means the writer never said — and the
 * callers below all have something sensible to do with that (print the bare
 * number, refuse the ramp) which is better than inventing "dB".
 */
export function unitFor(
  units: Record<string, string> | undefined,
  name: string,
): string {
  return units?.[name] ?? "";
}

/**
 * The subset of `order` whose values are decibels, in the order given.
 *
 * This is what replaced the all-or-nothing gate. The unit used to be one
 * string per container, so `beb-exposure`'s `"mixed"` refused Lden and Lnight
 * along with the six counts they sat beside — the run drew nothing at all.
 * Now each channel answers for itself and only the counts are withheld.
 *
 * Iterating `order` and not the map keeps the container's declared order,
 * which is what the indicator picker shows.
 */
export function levelIndicators(
  order: readonly string[],
  units: Record<string, string> | undefined,
): string[] {
  return order.filter((name) => declaresLevels(unitFor(units, name)));
}

/**
 * A container as it may arrive: the current shape, or the one written before
 * the unit was per channel.
 *
 * Runs persist — on disk under `.noise/runs/`, and in IndexedDB for browser
 * mode — and nothing rewrites them, so the reader is what has to cope. Go does
 * this in `LoadReceiverTableJSON` and `RasterMetadata.UnmarshalJSON`; this is
 * the same expansion on the side that reads the artifact as JSON and asserts a
 * type over it, which performs no conversion of its own.
 *
 * Without it the upgrade is silent and total: `units` comes back `undefined`,
 * `levelIndicators` finds no decibel indicator, and every existing run loses
 * its circles, its picker and its raster — with the panel reporting that the
 * table holds no decibels, which is exactly wrong.
 */
type LegacyUnitContainer = {
  units?: Record<string, string>;
  unit?: string;
};

/**
 * Fills in `units` from a legacy scalar `unit`, spread over the channels the
 * container declares.
 *
 * Returns the value unchanged when it already carries units, so a current
 * container is not copied on every render. Lossless for every container ever
 * written: the scalar really was true of all of them — the one writer whose
 * channels disagreed said `"mixed"`, and that is the case the per-channel
 * field exists to stop.
 */
export function withLegacyUnits<T extends LegacyUnitContainer>(
  container: T,
  channels: readonly string[] | undefined,
): T {
  if (container.units !== undefined) return container;

  const scalar = container.unit;
  if (scalar === undefined || scalar === "" || channels === undefined) {
    return container;
  }

  return {
    ...container,
    units: Object.fromEntries(channels.map((name) => [name, scalar])),
  };
}

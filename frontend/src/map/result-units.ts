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

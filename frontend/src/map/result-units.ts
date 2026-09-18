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

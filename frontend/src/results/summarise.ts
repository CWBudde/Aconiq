/**
 * The min/max/mean the receiver table prints above itself, counted in one pass.
 *
 * The obvious spelling — `Math.min(...records.map(…))` — is not merely slow, it
 * is a hard ceiling. A spread call places one argument per stack slot, so past
 * roughly 125 000 elements V8 answers `RangeError: Maximum call stack size
 * exceeded` and the whole tab renders its error state instead of a table. The
 * auto receiver grid is capped at 250 000 (`run_options.go`) and the browser
 * kernel's own grid builder is capped at nothing at all, so that ceiling sits
 * well inside the range this app is expected to produce.
 *
 * One loop over the records, with the accumulators held per indicator, has no
 * argument list and therefore no ceiling.
 */

/** The part of a receiver record a summary reads. */
export interface SummarisableRecord {
  values: Record<string, number>;
}

/** One card's worth of numbers. */
export interface IndicatorSummary {
  ind: string;
  min: number;
  max: number;
  mean: number;
}

/**
 * Summarise each indicator across every record, in the order the indicators
 * are given.
 *
 * A record missing an indicator reads as 0 rather than being skipped, which is
 * the same `?? 0` the table cells and the sort comparator apply — the three
 * have to agree, or a card describes a column the user cannot see.
 *
 * An empty table gives every indicator a zeroed card rather than no card, so
 * the row of cards keeps its shape while a filter is being typed.
 *
 * The mean is a plain left-to-right sum, as it was before: these are display
 * figures for one screen, not a normative reduction, and a compensated sum
 * here would change the digits shown without changing what they say.
 */
export function summariseIndicators(
  records: readonly SummarisableRecord[],
  indicators: readonly string[],
): IndicatorSummary[] {
  const acc = indicators.map((ind) => ({
    ind,
    min: Number.POSITIVE_INFINITY,
    max: Number.NEGATIVE_INFINITY,
    sum: 0,
  }));

  for (const record of records) {
    for (const a of acc) {
      const value = record.values[a.ind] ?? 0;
      if (value < a.min) a.min = value;
      if (value > a.max) a.max = value;
      a.sum += value;
    }
  }

  const count = records.length;
  return acc.map(({ ind, min, max, sum }) =>
    count === 0
      ? { ind, min: 0, max: 0, mean: 0 }
      : { ind, min, max, mean: sum / count },
  );
}

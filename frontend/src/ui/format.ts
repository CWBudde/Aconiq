import { getLocale } from "@/i18n/runtime";

/**
 * Locale-aware formatters for the values the pages print: levels, plain
 * numbers, coordinates, timestamps and durations.
 *
 * Every formatter reads the active paraglide locale at call time, so the
 * language toggle and the E2E locale pin both apply, and every one returns
 * an en dash for a value that is not a finite number or a valid date, so a
 * missing value never renders as "NaN" or "Invalid Date".
 */

/** The placeholder for a value that cannot be formatted. */
export const MISSING_VALUE = "–";

/** Narrow no-break space (U+202F): joins a number to its unit. */
const NARROW_NBSP = " ";

function numberFormat(digits: number): Intl.NumberFormat {
  return new Intl.NumberFormat(getLocale(), {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
}

/**
 * A plain number with a fixed number of fraction digits, in the active
 * locale ("1.234,5" in `de`, "1,234.5" in `en`).
 */
export function formatNumber(value: number, digits = 1): string {
  if (!Number.isFinite(value)) return MISSING_VALUE;
  return numberFormat(digits).format(value);
}

/**
 * A sound level with its unit: "45,3 dB(A)" in `de`, "45.3 dB(A)" in `en`.
 * The number and the unit are joined by a narrow no-break space so they
 * never wrap apart.
 */
export function formatLevel(value: number, unit = "dB(A)", digits = 1): string {
  if (!Number.isFinite(value)) return MISSING_VALUE;
  return `${numberFormat(digits).format(value)}${NARROW_NBSP}${unit}`;
}

/** A coordinate component as a plain locale-formatted number. */
export function formatCoordinate(value: number, digits = 2): string {
  return formatNumber(value, digits);
}

function parseDate(iso: string | Date): Date | null {
  const date = iso instanceof Date ? iso : new Date(iso);
  return Number.isNaN(date.getTime()) ? null : date;
}

/** Date and time, medium date and short time ("04.03.2026, 13:05"). */
export function formatDateTime(iso: string | Date): string {
  const date = parseDate(iso);
  if (date === null) return MISSING_VALUE;
  return new Intl.DateTimeFormat(getLocale(), {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(date);
}

/** Time of day with seconds ("13:05:07"). */
export function formatTime(iso: string | Date): string {
  const date = parseDate(iso);
  if (date === null) return MISSING_VALUE;
  return new Intl.DateTimeFormat(getLocale(), {
    timeStyle: "medium",
  }).format(date);
}

type DurationUnit = "millisecond" | "second" | "minute";

function unitFormat(unit: DurationUnit): Intl.NumberFormat {
  return new Intl.NumberFormat(getLocale(), {
    style: "unit",
    unit,
    unitDisplay: "short",
    maximumFractionDigits: 0,
  });
}

/**
 * A duration in milliseconds at the coarsest unit that still reads well:
 * below a second the milliseconds ("123 ms"), below a minute the rounded
 * seconds ("12 sec" / "12 Sek."), otherwise whole minutes and the remaining
 * seconds ("2 min 5 sec" / "2 Min. 5 Sek.").
 */
export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return MISSING_VALUE;
  if (ms < 1000) return unitFormat("millisecond").format(ms);
  if (ms < 60_000) return unitFormat("second").format(Math.round(ms / 1000));
  const minutes = Math.floor(ms / 60_000);
  const seconds = Math.round((ms % 60_000) / 1000);
  return `${unitFormat("minute").format(minutes)} ${unitFormat("second").format(seconds)}`;
}

/**
 * The duration between two timestamps, as `formatDuration`. An unparsable
 * or reversed pair yields the missing-value dash.
 */
export function formatDurationBetween(
  startIso: string | Date,
  endIso: string | Date,
): string {
  const start = parseDate(startIso);
  const end = parseDate(endIso);
  if (start === null || end === null) return MISSING_VALUE;
  return formatDuration(end.getTime() - start.getTime());
}

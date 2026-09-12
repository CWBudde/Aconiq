import { afterEach, describe, expect, it } from "vitest";
import { getLocale, overwriteGetLocale, type Locale } from "@/i18n/runtime";
import {
  MISSING_VALUE,
  formatCoordinate,
  formatDateTime,
  formatDuration,
  formatDurationBetween,
  formatLevel,
  formatNumber,
  formatTime,
} from "./format";

const originalGetLocale = getLocale;

function useLocale(locale: Locale) {
  overwriteGetLocale(() => locale);
}

afterEach(() => {
  overwriteGetLocale(originalGetLocale);
});

// The narrow no-break space that joins a number to its unit.
const NNBSP = " ";

// jsdom runs on Node's full ICU, so every expected string below is the exact
// output; a locale regression shows as a diff, not as a skipped assertion.
// Dates are built from explicit local-time components so the assertions do
// not depend on the time zone the test runs in.
const sample = new Date(2026, 2, 4, 13, 5, 7);

describe("formatNumber", () => {
  it("uses the decimal and grouping separators of the locale", () => {
    useLocale("en");
    expect(formatNumber(1234.5, 1)).toBe("1,234.5");
    useLocale("de");
    expect(formatNumber(1234.5, 1)).toBe("1.234,5");
  });

  it("pads and rounds to the requested digits", () => {
    useLocale("en");
    expect(formatNumber(45, 2)).toBe("45.00");
    expect(formatNumber(45.256, 2)).toBe("45.26");
    expect(formatNumber(7.9, 0)).toBe("8");
  });

  it("returns the dash for a value that is not a finite number", () => {
    useLocale("en");
    expect(formatNumber(Number.NaN)).toBe(MISSING_VALUE);
    expect(formatNumber(Number.POSITIVE_INFINITY)).toBe(MISSING_VALUE);
    expect(formatNumber(Number.NEGATIVE_INFINITY)).toBe(MISSING_VALUE);
  });
});

describe("formatLevel", () => {
  it("joins the level and its unit with a narrow no-break space", () => {
    useLocale("en");
    expect(formatLevel(45.3)).toBe(`45.3${NNBSP}dB(A)`);
    useLocale("de");
    expect(formatLevel(45.3)).toBe(`45,3${NNBSP}dB(A)`);
  });

  it("accepts another unit and digit count", () => {
    useLocale("en");
    expect(formatLevel(62.125, "dB", 2)).toBe(`62.13${NNBSP}dB`);
  });

  it("never renders NaN", () => {
    useLocale("de");
    expect(formatLevel(Number.NaN)).toBe(MISSING_VALUE);
  });
});

describe("formatCoordinate", () => {
  it("formats a plain number with two digits by default", () => {
    useLocale("en");
    expect(formatCoordinate(512345.678)).toBe("512,345.68");
    useLocale("de");
    expect(formatCoordinate(512345.678)).toBe("512.345,68");
  });

  it("returns the dash for a non-finite value", () => {
    useLocale("en");
    expect(formatCoordinate(Number.NaN)).toBe(MISSING_VALUE);
  });
});

describe("formatDateTime", () => {
  it("prints a medium date and a short time per locale", () => {
    useLocale("en");
    expect(formatDateTime(sample)).toBe("Mar 4, 2026, 1:05 PM");
    useLocale("de");
    expect(formatDateTime(sample)).toBe("04.03.2026, 13:05");
  });

  it("accepts an ISO string", () => {
    useLocale("de");
    expect(formatDateTime(sample.toISOString())).toBe("04.03.2026, 13:05");
  });

  it("returns the dash for an unparsable timestamp", () => {
    useLocale("en");
    expect(formatDateTime("not a date")).toBe(MISSING_VALUE);
    expect(formatDateTime("")).toBe(MISSING_VALUE);
  });
});

describe("formatTime", () => {
  it("prints the time of day with seconds per locale", () => {
    useLocale("en");
    expect(formatTime(sample)).toBe("1:05:07 PM");
    useLocale("de");
    expect(formatTime(sample)).toBe("13:05:07");
  });

  it("returns the dash for an unparsable timestamp", () => {
    useLocale("de");
    expect(formatTime("nope")).toBe(MISSING_VALUE);
  });
});

describe("formatDuration", () => {
  it("prints milliseconds below one second", () => {
    useLocale("en");
    expect(formatDuration(123)).toBe("123 ms");
    useLocale("de");
    expect(formatDuration(123)).toBe("123 ms");
  });

  it("prints rounded seconds below one minute", () => {
    useLocale("en");
    expect(formatDuration(12_400)).toBe("12 sec");
    expect(formatDuration(12_600)).toBe("13 sec");
    useLocale("de");
    expect(formatDuration(12_400)).toBe("12 Sek.");
  });

  it("prints minutes and the remaining seconds from one minute on", () => {
    useLocale("en");
    expect(formatDuration(125_000)).toBe("2 min 5 sec");
    expect(formatDuration(60_000)).toBe("1 min 0 sec");
    useLocale("de");
    expect(formatDuration(125_000)).toBe("2 Min. 5 Sek.");
  });

  it("returns the dash for a negative or non-finite duration", () => {
    useLocale("en");
    expect(formatDuration(-1)).toBe(MISSING_VALUE);
    expect(formatDuration(Number.NaN)).toBe(MISSING_VALUE);
    expect(formatDuration(Number.POSITIVE_INFINITY)).toBe(MISSING_VALUE);
  });
});

describe("formatDurationBetween", () => {
  it("formats the span between two timestamps", () => {
    useLocale("en");
    expect(
      formatDurationBetween("2026-03-04T13:05:07Z", "2026-03-04T13:07:12Z"),
    ).toBe("2 min 5 sec");
  });

  it("returns the dash when an end is unparsable or the span is reversed", () => {
    useLocale("en");
    expect(formatDurationBetween("", "2026-03-04T13:07:12Z")).toBe(
      MISSING_VALUE,
    );
    expect(
      formatDurationBetween("2026-03-04T13:07:12Z", "2026-03-04T13:05:07Z"),
    ).toBe(MISSING_VALUE);
  });
});

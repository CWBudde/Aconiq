/**
 * The `aconiq` commands the UI hands to the user.
 *
 * Where the UI stops, it shows the exact command that carries on. That command
 * has to match the binary: a flag the CLI never grew, or a `--format` value it
 * rejects, is a copied line that fails in the user's terminal with this UI's
 * name on it. So the strings are built here, beside the rest of the backend
 * contract and under test, rather than assembled inline in whichever page
 * happened to need one.
 *
 * Mirrors the flag surface of `backend/internal/app/cli/export.go`.
 */

/** The `--format` values `aconiq export` accepts, in the order it lists them. */
export const EXPORT_FORMATS = [
  "geotiff",
  "cog",
  "gpkg",
  "contour-geojson",
  "contour-gpkg",
] as const;

export type ExportFormat = (typeof EXPORT_FORMATS)[number];

/**
 * Stands in for a run the user has not picked yet.
 *
 * Not cosmetic: `aconiq export` defaults `--run-id` to the *latest* run, so a
 * command emitted with a blank id acts on a different run than the one on
 * screen. `exportCommand` substitutes this rather than emitting one.
 */
export const RUN_ID_PLACEHOLDER = "<run-id>";

export interface ExportCommandOptions {
  /** `--format`, comma-separated in the order given. Omitted when empty. */
  formats?: readonly ExportFormat[];
  /** `--pdf`: compile `report.pdf` with Typst beside the offline report. */
  pdf?: boolean;
}

/**
 * `aconiq export` for one run, carrying the flags the caller asks for.
 *
 * Flags are emitted in the order `export.go` registers them, so a copied line
 * reads in the same sequence as `aconiq export --help`.
 *
 * Deliberately absent: `--out`, `--target-crs`, `--contour-interval` and
 * `--skip-report`, which no call site offers. `--skip-report` in particular is
 * rejected by the CLI together with `--pdf`, and importing that invariant here
 * for no consumer would be a second place to keep it correct.
 */
export function exportCommand(
  runId: string,
  options: ExportCommandOptions = {},
): string {
  const trimmed = runId.trim();
  const id = trimmed === "" ? RUN_ID_PLACEHOLDER : trimmed;

  const parts = ["aconiq export", `--run-id ${id}`];

  const formats = options.formats ?? [];
  if (formats.length > 0) parts.push(`--format ${formats.join(",")}`);
  if (options.pdf === true) parts.push("--pdf");

  return parts.join(" ");
}

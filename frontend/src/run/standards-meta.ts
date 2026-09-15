/**
 * What to call a standards module on screen.
 *
 * `GET /api/v1/standards` carries an `id` and an English `description` but no
 * display name, so the names live here. They are proper nouns — "RLS-19",
 * "ISO 9613-2" — and identical in every locale, so they are literals rather
 * than message keys: routing them through the catalogues would double the
 * translation diff to translate nothing. `standards-meta.test.ts` asserts every
 * registered id has a row, since the locale parity test cannot see a `.ts`
 * table.
 *
 * **A label may never assert more evidence than the module has.** The tier
 * travels on `StandardDescriptor.evidence_tier` and `EvidenceTierBadge` shows
 * it — but only in two of the five places a name is rendered: the run dialog's
 * select and its summary. The filter bar, the run detail header and the run
 * list row have no badge and no cheap way to get one, because `RunSummary`
 * carries no tier at all. So the limit rides in the name itself: a scaffold is
 * labelled "(Gerüst)", which is the word `docs/conformance/
 * cnossos-umfangserklaerung.md` uses for exactly these modules — they "setzen
 * die von ihnen benannten Vorschriften nicht um". `bub-road` is
 * "BUB Straße (Gerüst)", never "Berechnungsmethode BUB — Straßenverkehr".
 */

const SCAFFOLD_SUFFIX = " (Gerüst)";

const STANDARD_NAMES: Record<string, string> = {
  // Normative: real structure, coefficients and tables from the published norm.
  "rls19-road": "RLS-19 Straße",
  schall03: "Schall 03 (2014) Schiene",
  iso9613: "ISO 9613-2",

  // Preview: the aggregation is sound, the levels it consumes are not.
  "beb-exposure": "BEB Belastetenermittlung (Vorschau)",

  // Scaffolds. The name states the target; the suffix states that it is only a
  // target. See the module doc comment above.
  "cnossos-road": `CNOSSOS-EU Straße${SCAFFOLD_SUFFIX}`,
  "cnossos-rail": `CNOSSOS-EU Schiene${SCAFFOLD_SUFFIX}`,
  "cnossos-industry": `CNOSSOS-EU Industrie${SCAFFOLD_SUFFIX}`,
  "cnossos-aircraft": `CNOSSOS-EU Luftverkehr${SCAFFOLD_SUFFIX}`,
  "bub-road": `BUB Straße${SCAFFOLD_SUFFIX}`,
  "bub-rail": `BUB Schiene${SCAFFOLD_SUFFIX}`,
  "bub-industry": `BUB Industrie${SCAFFOLD_SUFFIX}`,
  "buf-aircraft": `BUF Luftverkehr${SCAFFOLD_SUFFIX}`,

  // An intentional test fixture, and it says so.
  "dummy-freefield": "Freifeld (Testmodul)",
};

/** Every id this build knows a name for. Exported for the coverage test. */
export const KNOWN_STANDARD_IDS = Object.keys(STANDARD_NAMES);

/**
 * The display name for a standard id, falling back to the id itself.
 *
 * The fallback is the design, not a gap: a backend newer than this build may
 * register a module nobody here has named, and printing its id is honest.
 */
export function getStandardLabel(standardId: string): string {
  return STANDARD_NAMES[standardId] ?? standardId;
}

/**
 * What to call a standards module on screen.
 *
 * `GET /api/v1/standards` carries an `id` and an English `description` but no
 * display name, so the names live here. They are proper nouns — "RLS-19",
 * "ISO 9613-2" — and identical in every locale, so they are literals rather
 * than message keys: routing them through the catalogues would double the
 * translation diff to translate nothing.
 *
 * **A label may never assert more evidence than the module has**, and the
 * qualifier that keeps it honest is *derived*, never tabulated. The tier is a
 * field the backend publishes on every descriptor — `evidence_tier` — and
 * `AGENTS.md` is explicit that it is the machine-readable form of the module's
 * compliance boundary, from which the free text is derived rather than
 * maintained as a second, parallel signal. A hard-coded "(Gerüst)" beside
 * `bub-road` is exactly that second signal: it keeps making the claim after
 * the backend has stopped, or stops making it before the backend does.
 *
 * So the table below holds names only. `getStandardLabel` appends the
 * qualifier when it is given a tier, spelling it with the same catalogue keys
 * `EvidenceTierBadge` uses, so the two can never drift apart. A caller that
 * has the descriptor passes `evidence_tier` straight in; a caller that has
 * only a `RunSummary` — which carries no tier — resolves one through
 * `useStandardLabel`.
 *
 * Given no tier the name stands alone, which is the same thing
 * `EvidenceTierBadge` does with an absent tier: an absent claim is not a tier,
 * and inventing one from a local table is how this went wrong the first time.
 */

import { parseEvidenceTier } from "@/api/evidence-tier";
import { m } from "@/i18n/messages";

const STANDARD_NAMES: Record<string, string> = {
  // Normative: real structure, coefficients and tables from the published norm.
  "rls19-road": "RLS-19 Straße",
  schall03: "Schall 03 (2014) Schiene",
  iso9613: "ISO 9613-2",

  // Preview: the aggregation is sound, the levels it consumes are not.
  "beb-exposure": "BEB Belastetenermittlung",

  // Scaffolds. The name states the target and nothing more; that it is only a
  // target is the tier's job to say. See docs/conformance/
  // cnossos-umfangserklaerung.md — these modules "setzen die von ihnen
  // benannten Vorschriften nicht um".
  "cnossos-road": "CNOSSOS-EU Straße",
  "cnossos-rail": "CNOSSOS-EU Schiene",
  "cnossos-industry": "CNOSSOS-EU Industrie",
  "cnossos-aircraft": "CNOSSOS-EU Luftverkehr",
  "bub-road": "BUB Straße",
  "bub-rail": "BUB Schiene",
  "bub-industry": "BUB Industrie",
  "buf-aircraft": "BUF Luftverkehr",

  // An intentional test fixture. Its tier says so.
  "dummy-freefield": "Freifeld",
};

/** Every id this build knows a name for. Exported for the coverage test. */
export const KNOWN_STANDARD_IDS = Object.keys(STANDARD_NAMES);

/**
 * The parenthesised limit a tier earns, in the reader's language.
 *
 * `normative` earns none: the name is the claim and the evidence backs it.
 * Neither does an absent tier, nor one this build does not recognise — an
 * unqualified name is a weaker statement than a wrong qualifier.
 */
function tierQualifier(evidenceTier: string | undefined): string {
  switch (parseEvidenceTier(evidenceTier)) {
    case "preview":
      return ` (${m.evidence_tier_preview()})`;
    case "scaffold":
      return ` (${m.evidence_tier_scaffold()})`;
    case "test-fixture":
      return ` (${m.evidence_tier_test_fixture()})`;
    default:
      return "";
  }
}

/**
 * The display name for a standard id, falling back to the id itself.
 *
 * The fallback is the design, not a gap: a backend newer than this build may
 * register a module nobody here has named, and printing its id is honest.
 *
 * `evidenceTier` is the descriptor's own `evidence_tier`. Pass it wherever it
 * is in hand.
 */
export function getStandardLabel(
  standardId: string,
  evidenceTier?: string,
): string {
  return (
    (STANDARD_NAMES[standardId] ?? standardId) + tierQualifier(evidenceTier)
  );
}

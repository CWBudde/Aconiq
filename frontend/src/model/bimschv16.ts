// 16. BImSchV Gebietskategorien, for the receiver editor.
//
// This mirrors backend/internal/assessment/bimschv16/assessment.go: the four
// `AreaCategory` constants and the wording `AreaCategoryLabelDE` returns for
// each. `bimschv16.test.ts` pins both against that file, and pins the property
// name against `categoryFromFeature`, so the map cannot write a category the
// assessment does not read.

/**
 * The property a receiver carries its Gebietskategorie in.
 *
 * `categoryFromFeature` tries five spellings in order and this one is first,
 * so a value written here outranks any older spelling the same receiver still
 * carries. The editor deliberately reads and writes this key alone: the other
 * four arrive with German values (`wohngebiet`, `mischgebiet`, …) that
 * `ParseAreaCategory` maps and this enum does not list, and mapping them here
 * would be a second parser free to drift from the Go one.
 */
export const PROP_BIMSCHV16_AREA_CATEGORY = "bimschv16_area_category";

/**
 * The canonical values, in the order the Go const block declares them — which
 * is also ascending threshold order, from the quietest category to the
 * loudest.
 *
 * `ParseAreaCategory` also accepts the German spellings
 * `docs/geojson-schema-v1.md` shows, but these are the values the Go type owns
 * and therefore the safer thing to write into a model.
 */
export const BIMSCHV16_AREA_CATEGORIES = [
  "hospital_school_kurheim_altenheim",
  "residential",
  "mixed",
  "commercial",
] as const;

export type BimSchV16AreaCategory = (typeof BIMSCHV16_AREA_CATEGORIES)[number];

/**
 * The German label per category, word for word from `AreaCategoryLabelDE`.
 *
 * German in both locales, because these are the statutory categories of the
 * 16. BImSchV rather than UI copy — the same reason the panel shows a
 * Schall 03 Fahrbahnart as `schwellengleis` in English too. Translating
 * "Kern-, Dorf- oder Mischgebiet" would invent a category the law does not
 * name, and the assessment these values feed is German regardless.
 */
export const BIMSCHV16_AREA_CATEGORY_LABELS_DE: Readonly<
  Record<BimSchV16AreaCategory, string>
> = {
  hospital_school_kurheim_altenheim:
    "Krankenhaus, Schule, Kurheim oder Altenheim",
  residential: "Wohngebiet oder Kleinsiedlungsgebiet",
  mixed: "Kern-, Dorf- oder Mischgebiet",
  commercial: "Gewerbegebiet",
};

/**
 * The label for a stored value, falling back to the value itself — as
 * `AreaCategoryLabelDE` does, so a category imported under a spelling this
 * build does not know is shown rather than blanked.
 */
export function bimschv16AreaCategoryLabel(category: string): string {
  // Read through a widened type rather than a cast to the union: the cast
  // would tell the compiler every string is a known category, and the fallback
  // it then calls unreachable is the whole point of this function.
  const labels: Record<string, string | undefined> =
    BIMSCHV16_AREA_CATEGORY_LABELS_DE;
  return labels[category] ?? category;
}

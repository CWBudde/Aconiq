import { m } from "@/i18n/messages";

const STANDARD_LABELS: Record<string, () => string> = {
  upstream_mapping_standard: m.standard_upstream_mapping_standard,
};

const STANDARD_DESCRIPTIONS: Record<string, () => string> = {
  upstream_mapping_standard: m.standard_upstream_mapping_standard_description,
};

export function getStandardLabel(standardId: string): string {
  return STANDARD_LABELS[standardId]?.() ?? standardId;
}

export function getStandardDescription(
  standardId: string,
  fallback: string,
): string {
  return STANDARD_DESCRIPTIONS[standardId]?.() ?? fallback;
}

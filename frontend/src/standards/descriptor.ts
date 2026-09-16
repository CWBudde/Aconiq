/**
 * What a standards module says about itself: its identity, its evidence tier,
 * and the parameters a run of it takes.
 *
 * It lives in its own module rather than under `api/` because both backends
 * publish it and neither owns it. `aconiq serve` answers `GET
 * /api/v1/standards`, the WASM kernel answers `aconiq.standards()`, and both
 * encode it through the same Go package (`internal/standards/descriptorjson`).
 * Declaring the shape under `api/` would have made `wasm/kernel.ts` import from
 * `api/`, which `api/browser-backend.ts` imports from in turn.
 *
 * `api/client.ts` re-exports these, so an existing import site keeps working.
 */

export interface ParameterDefinition {
  name: string;
  kind: "string" | "bool" | "int" | "float";
  /**
   * Physical unit of the value as a short symbol — "m", "km/h", "dB",
   * "dB/km", "1/h", "1/km", "%", "°C", "°". Absent when the parameter is
   * dimensionless (a share, a factor, a count) or not numeric at all.
   *
   * The symbol is the conventional SI-style spelling, not the suffix the
   * parameter name happens to use: `speed_pkw_kph` carries "km/h". Declared by
   * `framework.ParameterDefinition.Unit` in the Go modules and published by
   * both backends; optional because older backends omit it.
   */
  unit?: string;
  /**
   * Whether a run must state this parameter. Optional: the Go encoding omits
   * nothing here, but a parameter that carries a default never needs stating,
   * and every rls19-road parameter does. Treat an absent value as false —
   * `parameter-field.tsx` marks only a truthy one.
   */
  required?: boolean;
  default_value?: string;
  description?: string;
  enum?: string[];
  min?: number;
  max?: number;
}

export interface ProfileInfo {
  name: string;
  supported_source_types: string[];
  supported_indicators: string[];
  parameters: ParameterDefinition[];
}

export interface VersionInfo {
  name: string;
  default_profile: string;
  profiles: ProfileInfo[];
}

export interface StandardDescriptor {
  /**
   * Which assessment question the standard answers: `planning` for an
   * individual project's approval case, `mapping` for area-wide strategic
   * noise mapping. Typed as a plain string because older backends omit it.
   */
  context?: string;
  id: string;
  description: string;
  default_version: string;
  versions: VersionInfo[];
  /**
   * How much a module's output can be trusted: `normative`, `preview`,
   * `scaffold` or `test-fixture`. Optional and deliberately typed as a plain
   * string — older backends omit the field, and newer ones may report a tier
   * this build does not know yet. Narrow it with `parseEvidenceTier` rather
   * than comparing raw strings.
   */
  evidence_tier?: string;
}

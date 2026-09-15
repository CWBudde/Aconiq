import { useMemo, useState } from "react";
import type {
  ProfileInfo,
  StandardDescriptor,
  VersionInfo,
} from "@/api/client";

export interface RunSetupSelection {
  /** The standard actually in force: the explicit choice, or the first offered. */
  standardId: string;
  version: string;
  profile: string;
  standard: StandardDescriptor | undefined;
  versionInfo: VersionInfo | undefined;
  profileInfo: ProfileInfo | undefined;
  /** Parameter values by name, seeded from the selected profile's defaults. */
  params: Record<string, string>;
  selectStandard: (id: string) => void;
  selectVersion: (name: string) => void;
  selectProfile: (name: string) => void;
  setParam: (name: string, value: string) => void;
}

function defaultParams(profile: ProfileInfo): Record<string, string> {
  const out: Record<string, string> = {};
  for (const p of profile.parameters) {
    out[p.name] = p.default_value ?? "";
  }
  return out;
}

/**
 * Standard → version → profile → parameters, as one cascade.
 *
 * Each level stores the *explicit* choice and falls back to the level above's
 * declared default, so "" means "whatever the backend says" rather than
 * "nothing". That is what lets a standards list arriving late, or changing
 * under the dialog, resolve to something sensible without an effect that
 * writes state.
 *
 * Choosing a level resets everything below it. A re-selected standard returns
 * to its own defaults rather than to the last choice made under it: the
 * parameters of a profile are only meaningful for that profile, and carrying
 * them across is how a run gets values the standard never offered.
 *
 * **The parameter seeding is render-phase on purpose, and must stay that way.**
 * `profileKey` is compared against the key the current `params` were seeded
 * for, and a mismatch re-seeds during the same render. An effect would work
 * too — and would paint the fields empty for one commit on the way. That is
 * visible, and `run.test.tsx` pins it with a `MutationObserver` that records
 * every value a field held: the assertion is `["999", "25"]`, not
 * `["999", "", "25"]`.
 */
export function useRunSetupSelection(
  standards: StandardDescriptor[] | undefined,
): RunSetupSelection {
  const [standardId, setStandardId] = useState("");
  const [version, setVersion] = useState("");
  const [profile, setProfile] = useState("");
  const [params, setParams] = useState<Record<string, string>>({});
  const [seededFor, setSeededFor] = useState("");

  const effectiveStandardId = standardId || standards?.[0]?.id || "";
  const standard = useMemo(
    () => standards?.find((s) => s.id === effectiveStandardId),
    [standards, effectiveStandardId],
  );

  const effectiveVersion = version || standard?.default_version || "";
  const versionInfo = useMemo(
    () => standard?.versions.find((v) => v.name === effectiveVersion),
    [standard, effectiveVersion],
  );

  const effectiveProfile = profile || versionInfo?.default_profile || "";
  const profileInfo = useMemo(
    () => versionInfo?.profiles.find((p) => p.name === effectiveProfile),
    [versionInfo, effectiveProfile],
  );

  const profileKey = `${effectiveStandardId}/${effectiveVersion}/${effectiveProfile}`;
  if (profileKey !== seededFor && profileInfo) {
    setSeededFor(profileKey);
    setParams(defaultParams(profileInfo));
  }

  return {
    standardId: effectiveStandardId,
    version: effectiveVersion,
    profile: effectiveProfile,
    standard,
    versionInfo,
    profileInfo,
    params,
    selectStandard: (id) => {
      setStandardId(id);
      setVersion("");
      setProfile("");
      setParams({});
      setSeededFor("");
    },
    selectVersion: (name) => {
      setVersion(name);
      setProfile("");
      setParams({});
      setSeededFor("");
    },
    selectProfile: (name) => {
      setProfile(name);
      setParams({});
      setSeededFor("");
    },
    setParam: (name, value) => {
      setParams((prev) => ({ ...prev, [name]: value }));
    },
  };
}

/**
 * What the run the dialog describes would cost in time, asked of the backend
 * while the reader is still choosing.
 *
 * The question costs a few seconds of the backend's time — it prepares the
 * run's scene and times a sample of its kernel — so it is not asked per
 * keystroke. The spec is debounced, and a spec that changes while an estimate
 * is out cancels it: TanStack Query aborts a query whose key the dialog has
 * moved on from, and the backend drops the request on that signal (the API
 * interrupts its probe; browser mode lets the kernel finish and discards it).
 * Nothing here blocks the dialog: a run can be started before, during or
 * without an estimate.
 */

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { backend, type RunSpec } from "./backend";
import type { RunEstimate } from "@/model/run-estimate";

/** How long the spec has to sit still before it is estimated. */
export const ESTIMATE_DEBOUNCE_MS = 600;

/** The debounced value: `value`, once it has not changed for `delayMs`. */
function useDebounced<T>(value: T, delayMs: number): T {
  const [settled, setSettled] = useState(value);

  useEffect(() => {
    const timer = setTimeout(() => {
      setSettled(value);
    }, delayMs);
    return () => {
      clearTimeout(timer);
    };
  }, [value, delayMs]);

  return settled;
}

/**
 * A small number standing for an object's identity, so a model revision can
 * sit in a query key without the key hashing the whole model.
 */
const revisions = new WeakMap<object, number>();
let nextRevision = 1;

function revisionOf(value: object): number {
  let revision = revisions.get(value);
  if (revision === undefined) {
    revision = nextRevision++;
    revisions.set(value, revision);
  }
  return revision;
}

export type RunEstimateState =
  | { status: "idle" }
  | { status: "estimating" }
  | { status: "failed" }
  | { status: "ready"; estimate: RunEstimate };

/**
 * The estimate of `spec`, or idle when `spec` is null — a dialog whose run
 * would be refused has nothing to estimate.
 *
 * `model` is any object whose identity changes whenever the model the run
 * would read changes — the store's feature list — so an edit to the model asks
 * again even though the parameters did not move.
 */
export function useRunEstimate(
  spec: RunSpec | null,
  model: object,
): RunEstimateState {
  // Keyed on the serialised spec, not the object: the dialog builds a fresh
  // one every render, and an identity that changes every render would never
  // settle.
  const key = spec === null ? null : JSON.stringify(spec);
  const settledKey = useDebounced(key, ESTIMATE_DEBOUNCE_MS);
  const settledRevision = useDebounced(revisionOf(model), ESTIMATE_DEBOUNCE_MS);

  const query = useQuery({
    // Its own root, not under `runs`: creating a run invalidates the runs
    // keys, and that must not re-measure an estimate nobody asked again for.
    queryKey: ["run-estimate", settledKey, settledRevision] as const,
    queryFn: ({ signal }) =>
      backend.estimateRun(JSON.parse(settledKey ?? "null") as RunSpec, signal),
    enabled: settledKey !== null && settledKey === key,
    // An estimate is a measurement of this machine now; reopening the dialog
    // measures again rather than quoting an old one.
    staleTime: 0,
    gcTime: 0,
    retry: false,
  });

  if (key === null) return { status: "idle" };
  // Current only if neither the spec nor the model moved since it was asked
  // for: an estimate of the model before an edit is not this model's.
  const current = settledKey === key && settledRevision === revisionOf(model);
  if (query.data !== undefined && current) {
    return { status: "ready", estimate: query.data };
  }
  if (query.isError && current) return { status: "failed" };
  return { status: "estimating" };
}

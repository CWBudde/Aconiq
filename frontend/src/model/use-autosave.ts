import { useEffect, useRef } from "react";
import { useModelStore } from "@/model/model-store";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";

export const DRAFT_KEY = "aconiq.model.draft";
const SAVE_DELAY_MS = 2000;

/** Returns true if a saved draft exists in localStorage. */
export function hasDraft(): boolean {
  try {
    return localStorage.getItem(DRAFT_KEY) !== null;
  } catch {
    return false;
  }
}

/** Reads and deserializes the saved draft, or returns null on failure. */
export interface ModelDraft {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
}

export function loadDraft(): ModelDraft | null {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as
      | ModelFeature[]
      | Partial<ModelDraft>
      | null;
    if (Array.isArray(parsed)) {
      return { features: parsed, receivers: [], calcArea: null };
    }
    if (parsed === null || typeof parsed !== "object") return null;
    return {
      features: Array.isArray(parsed.features) ? parsed.features : [],
      receivers: Array.isArray(parsed.receivers) ? parsed.receivers : [],
      // Drafts written before the calculation area was persisted have no
      // `calcArea` key at all.
      calcArea: parsed.calcArea ?? null,
    };
  } catch {
    return null;
  }
}

/** Removes the saved draft from localStorage. */
export function discardDraft(): void {
  try {
    localStorage.removeItem(DRAFT_KEY);
  } catch {
    // Storage unavailable — ignore.
  }
}

/**
 * Debounced autosave: when the model becomes dirty, saves the model to
 * localStorage after a short delay and calls `markClean()`. Also installs
 * a `beforeunload` guard while there are unsaved changes.
 *
 * Every piece of state that sets `dirty` must be both written here and listed
 * in the effect's dependencies. `calcArea` was neither: setting one marked the
 * model dirty, the effect then wrote a draft without it and called
 * `markClean()`, and the calculation area was silently lost.
 */
export function useAutosave(): void {
  const dirty = useModelStore((s) => s.dirty);
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const calcArea = useModelStore((s) => s.calcArea);
  const markClean = useModelStore((s) => s.markClean);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Debounced save to localStorage.
  useEffect(() => {
    if (!dirty) return;

    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => {
      try {
        localStorage.setItem(
          DRAFT_KEY,
          JSON.stringify({
            features,
            receivers,
            calcArea,
          } satisfies ModelDraft),
        );
        markClean();
      } catch {
        // Storage full or unavailable — skip silently.
      }
    }, SAVE_DELAY_MS);

    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [dirty, features, receivers, calcArea, markClean]);

  // Prevent accidental tab/window close when there are unsaved changes.
  useEffect(() => {
    if (!dirty) return;
    const handler = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", handler);
    return () => {
      window.removeEventListener("beforeunload", handler);
    };
  }, [dirty]);
}

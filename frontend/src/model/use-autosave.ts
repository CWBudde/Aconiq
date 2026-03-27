import { useEffect, useRef } from "react";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature, ModelReceiver } from "@/model/types";

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
}

export function loadDraft(): ModelDraft | null {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as ModelFeature[] | ModelDraft;
    if (Array.isArray(parsed)) {
      return { features: parsed, receivers: [] };
    }
    return {
      features: Array.isArray(parsed.features) ? parsed.features : [],
      receivers: Array.isArray(parsed.receivers) ? parsed.receivers : [],
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
 * Debounced autosave: when the model becomes dirty, saves features to
 * localStorage after a short delay and calls `markClean()`. Also installs
 * a `beforeunload` guard while there are unsaved changes.
 */
export function useAutosave(): void {
  const dirty = useModelStore((s) => s.dirty);
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
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
          JSON.stringify({ features, receivers } satisfies ModelDraft),
        );
        markClean();
      } catch {
        // Storage full or unavailable — skip silently.
      }
    }, SAVE_DELAY_MS);

    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, [dirty, features, receivers, markClean]);

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

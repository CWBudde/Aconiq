import { useEffect, useRef } from "react";
import { backend } from "@/api/backend";
import { useModelStore } from "@/model/model-store";
import type { CalcArea, ModelFeature, ModelReceiver } from "@/model/types";

export const DRAFT_KEY = "aconiq.model.draft";
/**
 * Stamped into every draft written. Bump it when the draft's shape changes
 * incompatibly; `loadDraft` refuses a version it does not know rather than
 * guessing at the fields, and still accepts the two shapes that predate the
 * stamp.
 */
export const DRAFT_VERSION = 1;
const SAVE_DELAY_MS = 2000;

/** Returns true if a saved draft exists that this build can restore. */
export function hasDraft(): boolean {
  // Defined by what `loadDraft` would return, not by whether the key exists:
  // a draft at a version this build refuses would otherwise get the banner
  // to offer a Restore that does nothing.
  return loadDraft() !== null;
}

/** Reads and deserializes the saved draft, or returns null on failure. */
export interface ModelDraft {
  features: ModelFeature[];
  receivers: ModelReceiver[];
  calcArea: CalcArea | null;
  /**
   * The project's receipt for this exact content: the hash `POST
   * /api/v1/model` answered with for the bytes these features produced.
   *
   * The safety property the whole hydration design rests on is that a draft
   * carrying a hash *is* the content that produced the file that hash names.
   * It holds because exactly one writer ever supplies one — `useProjectSync`,
   * in the branch that has already established the store did not move during
   * the request. Absent on every other draft, which is why it is optional
   * rather than nullable, and why `DRAFT_VERSION` does not move: an additive
   * optional field, exactly as `calcArea` was, and a bump would throw away
   * every draft already on disk.
   */
  hash?: string;
}

export function loadDraft(): ModelDraft | null {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (!raw) return null;
    const parsed = JSON.parse(raw) as
      | ModelFeature[]
      | (Partial<ModelDraft> & { version?: unknown })
      | null;
    // The oldest drafts were a bare feature array.
    if (Array.isArray(parsed)) {
      return { features: parsed, receivers: [], calcArea: null };
    }
    if (parsed === null || typeof parsed !== "object") {
      console.warn("Ignoring a saved draft that is not an object");
      return null;
    }
    // Drafts written before the version stamp carry no `version` key; those
    // are read as version 1, which has the same fields.
    if ("version" in parsed && parsed.version !== DRAFT_VERSION) {
      console.warn(
        `Ignoring a saved draft with version ${String(parsed.version)}, expected ${String(DRAFT_VERSION)}`,
      );
      return null;
    }
    return {
      features: Array.isArray(parsed.features) ? parsed.features : [],
      receivers: Array.isArray(parsed.receivers) ? parsed.receivers : [],
      // Drafts written before the calculation area was persisted have no
      // `calcArea` key at all.
      calcArea: parsed.calcArea ?? null,
      // Likewise for the hash, and for every draft the autosave writes. A
      // value that is not a string is dropped rather than carried: a draft
      // that claims a hash it cannot have would be restored as if it were the
      // project's own model.
      ...(typeof parsed.hash === "string" ? { hash: parsed.hash } : {}),
    };
  } catch {
    return null;
  }
}

/**
 * Writes the draft; returns whether it was stored. The one writer: the
 * debounced autosave and a successful project save both go through here, so
 * a draft can never be shaped differently depending on who wrote it.
 */
export function writeDraft(draft: ModelDraft): boolean {
  try {
    localStorage.setItem(
      DRAFT_KEY,
      JSON.stringify({ version: DRAFT_VERSION, ...draft }),
    );
    return true;
  } catch {
    // Storage full or unavailable — the draft is best-effort.
    return false;
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
 * Debounced autosave: when the model becomes dirty, writes the model to
 * localStorage after a short delay. Also installs a `beforeunload` guard while
 * `dirty` is set.
 *
 * The draft is written in both modes — it is crash recovery. Whether the
 * write also calls `markClean()` depends on what the project is:
 *
 * - When the backend runs against the saved model (`aconiq serve`), the
 *   project is the server's copy, and `dirty` means "differs from that copy".
 *   A localStorage draft changes nothing there, so the write must not clear
 *   the flag; only a successful `saveModel` does (`useProjectSync`). Marking
 *   clean here was what let the header claim a synced state the server had
 *   never seen.
 * - When runs read the in-memory store directly (browser mode), the draft is
 *   the only persistence there is — it *is* the project — so the write is
 *   the save, and clearing the flag here is honest.
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
      // Deliberately no `hash`. A draft the autosave writes is newer than the
      // project by construction — that is what `dirty` means here. Carrying
      // the last save's receipt forward onto newer edits would make the next
      // startup find a match, restore a divergent draft *clean*, and let the
      // workspace claim to be the project while differing from it.
      const written = writeDraft({ features, receivers, calcArea });
      if (written && !backend.capabilities.runsAgainstSavedModel) markClean();
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

import { useState } from "react";
import { History } from "lucide-react";
import { useModelStore } from "@/model/model-store";
import { loadDraft, discardDraft } from "@/model/use-autosave";
import { projectHydrationStore } from "@/model/use-project-hydration";
import { Button } from "@/ui/components/button";
import { m } from "@/i18n/messages";

/**
 * Shows a dismissable banner when a saved draft is worth offering. Lets the
 * user restore or discard it.
 *
 * Whether it is worth offering is decided by `useProjectHydration`, not here.
 * This component used to answer it itself, in a mount effect asking whether
 * the model was empty and a draft existed — but child effects run before
 * parent effects, so it answered before the hydration hook had looked at the
 * project, and it could only ever say yes to a draft over an empty map. A
 * draft that diverges from the project is precisely the one the user needs
 * offered, and after hydration the map is not empty.
 */
export function DraftBanner() {
  const loadModel = useModelStore((s) => s.loadModel);
  // Rendered only after the hydration decision, so a draft that turns out to
  // be the project's own model is never offered for a frame first.
  const started = projectHydrationStore((s) => s.started);
  const offered = projectHydrationStore((s) => s.draftOffered);
  const [dismissed, setDismissed] = useState(false);

  const visible = started && offered && !dismissed;
  if (!visible) return null;

  // The draft is not removed after a restore. `loadModel` marks the model
  // dirty, so the autosave rewrites the draft within its delay; discarding it
  // here left a window in which a reload lost the restored model, because
  // nothing had marked it unsaved and so nothing re-saved it.
  function handleRestore() {
    const draft = loadDraft();
    if (draft) {
      loadModel(draft);
    }
    setDismissed(true);
  }

  function handleDiscard() {
    discardDraft();
    setDismissed(true);
  }

  return (
    <div
      role="status"
      aria-label={m.banner_draft_recovery()}
      className="flex items-center gap-3 border-b bg-muted/60 px-4 py-2 text-sm"
    >
      <History className="h-4 w-4 shrink-0 text-muted-foreground" aria-hidden />
      <span className="flex-1 text-muted-foreground">
        {m.msg_unsaved_draft_found()}
      </span>
      <Button size="sm" variant="outline" onClick={handleDiscard}>
        {m.action_discard()}
      </Button>
      <Button size="sm" onClick={handleRestore}>
        {m.action_restore()}
      </Button>
    </div>
  );
}

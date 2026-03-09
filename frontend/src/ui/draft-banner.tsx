import { useEffect, useState } from "react";
import { History } from "lucide-react";
import { useModelStore } from "@/model/model-store";
import { hasDraft, loadDraft, discardDraft } from "@/model/use-autosave";
import { Button } from "@/ui/components/button";
import { m } from "@/i18n/messages";

/**
 * Shows a dismissable banner when a saved draft is found at startup and the
 * model is empty. Lets the user restore or discard the draft.
 */
export function DraftBanner() {
  const features = useModelStore((s) => s.features);
  const receivers = useModelStore((s) => s.receivers);
  const loadFeatures = useModelStore((s) => s.loadFeatures);
  const loadReceivers = useModelStore((s) => s.loadReceivers);
  const [visible, setVisible] = useState(false);

  // Check only on first mount — don't re-show after user interaction.
  useEffect(() => {
    if (features.length === 0 && receivers.length === 0 && hasDraft()) {
      setVisible(true);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (!visible) return null;

  function handleRestore() {
    const draft = loadDraft();
    if (draft) {
      loadFeatures(draft.features);
      loadReceivers(draft.receivers);
    }
    discardDraft();
    setVisible(false);
  }

  function handleDiscard() {
    discardDraft();
    setVisible(false);
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

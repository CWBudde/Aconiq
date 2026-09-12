import { useState } from "react";
import { AlertCircle, Check, Save } from "lucide-react";
import { modelValidationIssues } from "@/api/api-error";
import { useProjectSync } from "@/model/use-project-sync";
import { Button } from "@/ui/components/button";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/ui/components/dialog";
import { useGlobalShortcut } from "@/ui/hooks/use-global-shortcut";
import { m } from "@/i18n/messages";

/**
 * Header sync state: whether the workspace matches the project, and the
 * action that makes it so. Renders nothing where a project save is not a
 * thing (browser mode, or no project loaded) — there the draft is the
 * persistence and nothing needs saying.
 */
export function SaveStatus() {
  const { enabled, status, dirty, error, save } = useProjectSync();
  const [detailsOpen, setDetailsOpen] = useState(false);

  // Ctrl+S / Cmd+S. `preventDefault` runs whenever the shortcut is ours to
  // handle, dirty or not: a workspace must never pop the browser's save-page
  // dialog, and a clean one has simply nothing to do. The save follows
  // `dirty`, not `status`: after a failed save the status is "error" while
  // the model is still unsaved, and the shortcut is the retry. It fires from
  // inside a text field too: no input claims Ctrl+S for itself, and a user
  // who has just typed a height expects it to save.
  useGlobalShortcut(
    { key: "s", ctrl: true, allowInTextEntry: true, enabled },
    () => {
      if (dirty && status !== "saving") void save();
    },
  );

  if (!enabled) return null;

  const issues = status === "error" ? modelValidationIssues(error) : null;

  return (
    <div
      data-testid="save-status"
      data-status={status}
      className="mr-2 flex items-center gap-2 text-xs"
    >
      {/* One live region for every state, so a transition is announced. */}
      <span
        aria-live="polite"
        className={
          status === "error"
            ? "flex items-center gap-1 text-destructive"
            : "flex items-center gap-1 text-muted-foreground"
        }
      >
        {status === "clean" ? (
          <>
            <Check className="h-3.5 w-3.5" aria-hidden />
            {m.status_saved_to_project()}
          </>
        ) : status === "error" ? (
          <>
            <AlertCircle className="h-3.5 w-3.5" aria-hidden />
            {m.msg_save_failed()}
          </>
        ) : null}
      </span>

      {issues !== null ? (
        <Button
          size="sm"
          variant="ghost"
          onClick={() => {
            setDetailsOpen(true);
          }}
        >
          {m.action_show_details()}
        </Button>
      ) : null}

      {status !== "clean" ? (
        <Button
          size="sm"
          disabled={status === "saving"}
          onClick={() => {
            void save();
          }}
        >
          <Save aria-hidden />
          {status === "saving" ? m.status_saving() : m.action_save_to_project()}
        </Button>
      ) : null}

      {issues !== null ? (
        <Dialog open={detailsOpen} onOpenChange={setDetailsOpen}>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{m.dialog_title_model_invalid()}</DialogTitle>
              <DialogDescription>
                {m.dialog_desc_model_invalid()}
              </DialogDescription>
            </DialogHeader>
            <ul className="max-h-[60vh] space-y-2 overflow-y-auto text-sm">
              {issues.map((issue, i) => (
                <li
                  key={`${issue.code}-${issue.feature_id ?? ""}-${String(i)}`}
                  className="rounded-md border p-2"
                >
                  <div className="flex items-baseline gap-2">
                    <code className="text-xs text-muted-foreground">
                      {issue.code}
                    </code>
                    {issue.feature_id !== undefined ? (
                      <span className="text-xs text-muted-foreground">
                        {m.label_feature_id()}:{" "}
                        <code data-testid="issue-feature-id">
                          {issue.feature_id}
                        </code>
                      </span>
                    ) : null}
                  </div>
                  <p>{issue.message}</p>
                </li>
              ))}
            </ul>
          </DialogContent>
        </Dialog>
      ) : null}
    </div>
  );
}

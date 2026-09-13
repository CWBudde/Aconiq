import { AlertCircle } from "lucide-react";
import {
  projectHydrationStore,
  retryProjectHydration,
} from "@/model/use-project-hydration";
import { Button } from "@/ui/components/button";
import { m } from "@/i18n/messages";

/**
 * Says so when the workspace could not be read back from the project.
 *
 * Silence here is the failure the hydration hook exists to fix: an empty map
 * drawn over a populated project looks exactly like an empty project, and the
 * next save would write that emptiness back. A refused read has to be visible,
 * and it has to be retryable without a reload — the request can fail because
 * `aconiq serve` was restarted, which is over by the time the user reads this.
 */
export function HydrationBanner() {
  const status = projectHydrationStore((s) => s.status);
  const error = projectHydrationStore((s) => s.error);

  if (status !== "error") return null;

  return (
    <div
      role="alert"
      data-testid="hydration-error-banner"
      className="flex items-center gap-3 border-b border-warning/40 bg-warning/10 px-4 py-2 text-sm"
    >
      <AlertCircle className="h-4 w-4 shrink-0 text-warning" aria-hidden />
      <span className="flex-1">
        {m.msg_project_model_load_failed()}
        {error === null ? null : (
          <span className="ml-2 text-muted-foreground">{error.message}</span>
        )}
      </span>
      <Button size="sm" variant="outline" onClick={retryProjectHydration}>
        {m.action_retry()}
      </Button>
    </div>
  );
}

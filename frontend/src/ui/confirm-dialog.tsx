import type * as React from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/ui/components/alert-dialog";
import { buttonVariants } from "@/ui/components/button";
import { cn } from "@/ui/lib/utils";
import { m } from "@/i18n/messages";

export interface ConfirmDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  /**
   * What the reader is about to lose, in their terms — the run id, the counts
   * that will be discarded. A confirmation that only restates the button is
   * an obstacle rather than a safeguard, so say what happens.
   */
  description: React.ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  /** `destructive` paints the confirm action; use it when nothing can be undone. */
  tone?: "default" | "destructive";
  onConfirm: () => void;
}

/**
 * The one confirmation treatment for an action the reader cannot take back.
 *
 * Controlled, with no trigger slot: every call site already owns the state
 * that decides whether the action is offered at all, and a trigger slot would
 * pull the button's markup in here, where it would have to serve a banner
 * button, an editor footer and a page action at once.
 *
 * Radix gives this `role="alertdialog"`, a focus trap, Escape-to-cancel and
 * initial focus on Cancel, and it refuses to render without a title and a
 * description — which is the point: an alert dialog with no explanation is
 * the thing this component exists to prevent.
 */
export function ConfirmDialog({
  open,
  onOpenChange,
  title,
  description,
  confirmLabel,
  cancelLabel,
  tone = "default",
  onConfirm,
}: ConfirmDialogProps) {
  return (
    <AlertDialog open={open} onOpenChange={onOpenChange}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{title}</AlertDialogTitle>
          <AlertDialogDescription>{description}</AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>
            {cancelLabel ?? m.action_cancel()}
          </AlertDialogCancel>
          <AlertDialogAction
            className={cn(
              tone === "destructive" &&
                buttonVariants({ variant: "destructive" }),
            )}
            onClick={onConfirm}
          >
            {confirmLabel ?? m.action_confirm()}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

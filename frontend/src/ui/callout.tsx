import * as React from "react";
import type { LucideIcon } from "lucide-react";
import { Alert, AlertDescription, AlertTitle } from "@/ui/components/alert";
import { cn } from "@/ui/lib/utils";

export type CalloutVariant =
  | "neutral"
  | "info"
  | "warning"
  | "destructive"
  | "success";

// `title` is the callout's heading line, not the HTML tooltip attribute.
export interface CalloutProps extends Omit<
  React.HTMLAttributes<HTMLDivElement>,
  "title"
> {
  /** Tone. `neutral` is a quiet hint; the others carry a semantic colour. */
  variant?: CalloutVariant;
  /** Icon rendered in the Alert's icon slot, coloured like the text. */
  icon?: LucideIcon;
  /** Optional bold first line. */
  title?: React.ReactNode;
}

// Every variant maps to an Alert variant plus the ARIA role a reader should
// hear. A warning or a failure interrupts (`alert`); an informational or a
// success note is announced politely (`status`); a neutral hint is static
// content and gets no live-region role at all. A caller can still pass
// `role` explicitly to override.
const variantConfig: Record<
  CalloutVariant,
  {
    alert: "default" | "info" | "warning" | "destructive" | "success";
    role: "alert" | "status" | undefined;
    className?: string;
  }
> = {
  neutral: {
    alert: "default",
    role: undefined,
    className: "border bg-muted/30 text-muted-foreground",
  },
  info: { alert: "info", role: "status" },
  warning: { alert: "warning", role: "alert" },
  destructive: { alert: "destructive", role: "alert" },
  success: { alert: "success", role: "status" },
};

/**
 * A compact inline note: a hint, a warning, an error, a confirmation. Built
 * on `Alert` so the variants share one set of tokens, but sized for a form
 * or a detail pane (`text-xs`, `p-3`) rather than a page banner.
 */
export function Callout({
  variant = "neutral",
  icon: Icon,
  title,
  children,
  className,
  role,
  ...props
}: CalloutProps) {
  const cfg = variantConfig[variant];
  return (
    <Alert
      variant={cfg.alert}
      role={role ?? cfg.role}
      data-variant={variant}
      className={cn(
        "rounded-md p-3 text-xs [&>svg]:left-3 [&>svg]:top-3 [&>svg]:size-3.5 [&>svg+div]:translate-y-0 [&>svg~*]:pl-6",
        cfg.className,
        className,
      )}
      {...props}
    >
      {Icon ? <Icon aria-hidden="true" /> : null}
      {title != null ? (
        <AlertTitle className="mb-0.5 text-xs">{title}</AlertTitle>
      ) : null}
      <AlertDescription className="text-xs [&_p]:leading-normal">
        {children}
      </AlertDescription>
    </Alert>
  );
}

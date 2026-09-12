import { useEffect, useRef, useState } from "react";
import { Check, Copy } from "lucide-react";
import { toast } from "sonner";
import { m } from "@/i18n/messages";
import { Button, type ButtonProps } from "@/ui/components/button";
import { cn } from "@/ui/lib/utils";

/** How long the "copied" confirmation stays on the button. */
const COPIED_FLASH_MS = 1500;

export interface CopyButtonProps {
  /** The text written to the clipboard. */
  text: string;
  /** Button caption; defaults to the "Copy" action. */
  label?: string;
  size?: ButtonProps["size"];
  variant?: ButtonProps["variant"];
  className?: string;
}

/**
 * Copies `text` to the clipboard and flashes a confirmation for 1.5 s. A
 * refused write (no permission, no secure context) is reported as a toast
 * rather than swallowed. With `size="icon"` the caption becomes the
 * accessible name.
 */
export function CopyButton({
  text,
  label,
  size = "sm",
  variant = "outline",
  className,
}: CopyButtonProps) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current !== null) clearTimeout(timer.current);
    },
    [],
  );

  function handleCopy() {
    navigator.clipboard.writeText(text).then(
      () => {
        setCopied(true);
        if (timer.current !== null) clearTimeout(timer.current);
        timer.current = setTimeout(() => {
          setCopied(false);
          timer.current = null;
        }, COPIED_FLASH_MS);
      },
      () => {
        toast.error(m.msg_copy_failed());
      },
    );
  }

  const caption = copied ? m.status_copied() : (label ?? m.action_copy());
  const iconOnly = size === "icon";

  return (
    <Button
      type="button"
      variant={variant}
      size={size}
      onClick={handleCopy}
      aria-label={iconOnly ? caption : undefined}
      data-copied={copied ? "true" : undefined}
      className={cn(!iconOnly && "h-7 px-2 text-xs", className)}
    >
      {copied ? (
        <Check aria-hidden="true" className="text-success" />
      ) : (
        <Copy aria-hidden="true" />
      )}
      {iconOnly ? null : caption}
    </Button>
  );
}

export interface CopyFieldProps {
  /** Caption above the value. */
  label?: string;
  /** The value shown and copied. */
  value: string;
  /** Monospace value (a command, a path, an id). */
  mono?: boolean;
  className?: string;
}

/**
 * A read-only value with a copy button beside it: a CLI command, a path.
 */
export function CopyField({
  label,
  value,
  mono = true,
  className,
}: CopyFieldProps) {
  return (
    <div data-slot="copy-field" className={cn("space-y-1.5", className)}>
      {label != null ? <p className="text-xs font-medium">{label}</p> : null}
      <div className="flex items-center gap-3 rounded-md border bg-muted/30 px-3 py-2">
        <code
          title={value}
          className={cn(
            "min-w-0 flex-1 truncate text-xs",
            mono ? "font-mono" : "font-sans",
          )}
        >
          {value}
        </code>
        <CopyButton text={value} />
      </div>
    </div>
  );
}

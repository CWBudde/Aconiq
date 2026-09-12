import type * as React from "react";
import { Toaster as Sonner, type ToasterProps } from "sonner";

import { m } from "@/i18n/messages";
import { useTheme } from "@/ui/theme-provider";

// The app's own ThemeProvider drives the toast theme (upstream reads
// `next-themes`). The colours come from the design tokens through sonner's
// CSS variables, so `richColors` stays off; every toast gets a close button.
const Toaster = ({ toastOptions, ...props }: ToasterProps) => {
  const { theme } = useTheme();

  return (
    <Sonner
      theme={theme}
      className="toaster group"
      closeButton
      richColors={false}
      toastOptions={{
        closeButtonAriaLabel: m.action_close(),
        ...toastOptions,
      }}
      style={
        {
          "--normal-bg": "var(--popover)",
          "--normal-text": "var(--popover-foreground)",
          "--normal-border": "var(--border)",
        } as React.CSSProperties
      }
      {...props}
    />
  );
};

export { Toaster };

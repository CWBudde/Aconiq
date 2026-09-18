import { RouterProvider } from "react-router";
import { QueryClientProvider } from "@tanstack/react-query";
import { ThemeProvider } from "@/ui/theme-provider";
import { Toaster } from "@/ui/components/sonner";
import { queryClient } from "@/api/query-client";
import { useLocale } from "@/locale";
import { router } from "./routes";

export function App() {
  const locale = useLocale();

  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider defaultTheme="system">
        {/*
          The key is what re-renders the routes, and it is a remount on purpose.

          Paraglide resolves the locale per `m.*()` call, so every message under
          here is already correct the moment React renders it again — but React
          will not render it again on its own. A context would only reach its
          own consumers, and the seventy-eight modules that call `m` consume
          nothing; `RouterProvider` renders its route tree through a memo keyed
          on router state, so a plain re-render from above stops at that memo.

          What a remount costs is component state inside the routes: the map
          instance, an open dialog, the scroll position. What it does not cost
          is anything that matters more — the model and its command stack live
          at module scope, the query cache is this provider's and sits above the
          key, and the URL is the router's. Every one of those was lost by the
          `location.reload()` this replaces, along with a full re-download of
          the bundle and a second project hydration.
        */}
        <RouterProvider key={locale} router={router} />
        <Toaster />
      </ThemeProvider>
    </QueryClientProvider>
  );
}

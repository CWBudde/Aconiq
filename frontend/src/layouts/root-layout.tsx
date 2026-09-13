import { Suspense } from "react";
import { Outlet } from "react-router";
import { AppShell } from "@/ui/app-shell";
import { PageSkeleton } from "@/ui/page-skeleton";
import { ErrorBoundary } from "@/ui/error-boundary";
import { DraftBanner } from "@/ui/draft-banner";
import { HydrationBanner } from "@/ui/hydration-banner";
import { useAutosave } from "@/model/use-autosave";
import { useProjectHydration } from "@/model/use-project-hydration";

export function RootLayout() {
  // Before `useAutosave`, and here rather than on `/`: a reload can land on
  // any route, and every route draws the same model. Hydration leaves the
  // store clean, so the autosave below stays idle and a divergent draft
  // survives to be offered.
  useProjectHydration();
  useAutosave();
  return (
    <AppShell>
      <HydrationBanner />
      <DraftBanner />
      <ErrorBoundary>
        <Suspense fallback={<PageSkeleton />}>
          <Outlet />
        </Suspense>
      </ErrorBoundary>
    </AppShell>
  );
}

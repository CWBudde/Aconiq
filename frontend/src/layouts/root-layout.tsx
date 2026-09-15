import { Suspense } from "react";
import { Outlet } from "react-router";
import { AppShell } from "@/ui/app-shell";
import { PageSkeleton } from "@/ui/page-skeleton";
import { ErrorBoundary } from "@/ui/error-boundary";
import { DraftBanner } from "@/ui/draft-banner";
import { HydrationBanner } from "@/ui/hydration-banner";
import { useAutosave } from "@/model/use-autosave";
import {
  useHydrationSettled,
  useProjectHydration,
} from "@/model/use-project-hydration";

export function RootLayout() {
  // Before `useAutosave`, and here rather than on `/`: a reload can land on
  // any route, and every route draws the same model. Hydration leaves the
  // store clean, so the autosave below stays idle and a divergent draft
  // survives to be offered.
  useProjectHydration();
  useAutosave();
  const settled = useHydrationSettled();

  return (
    <AppShell>
      <HydrationBanner />
      <DraftBanner />
      <ErrorBoundary>
        <Suspense fallback={<PageSkeleton />}>
          {/*
            No route renders until hydration has settled. Every one of them is
            editable, and until the model arrives the store looks like an empty
            project: a user who imports or draws into that window makes the
            store dirty, hydration stands down rather than overwrite the edit,
            and the next save replaces the project's model with one built on
            top of nothing. The skeleton is the same one a lazily loaded page
            shows, so the wait looks like the wait the app already has.

            "Settled" includes "failed", so a backend that is down costs one
            banner and not the whole app.
          */}
          {settled ? <Outlet /> : <PageSkeleton />}
        </Suspense>
      </ErrorBoundary>
    </AppShell>
  );
}

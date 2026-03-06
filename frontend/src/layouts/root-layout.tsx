import { Suspense } from "react";
import { Outlet } from "react-router";
import { AppShell } from "@/ui/app-shell";
import { PageSkeleton } from "@/ui/page-skeleton";
import { ErrorBoundary } from "@/ui/error-boundary";
import { DraftBanner } from "@/ui/draft-banner";
import { useAutosave } from "@/model/use-autosave";

export function RootLayout() {
  useAutosave();
  return (
    <AppShell>
      <DraftBanner />
      <ErrorBoundary>
        <Suspense fallback={<PageSkeleton />}>
          <Outlet />
        </Suspense>
      </ErrorBoundary>
    </AppShell>
  );
}

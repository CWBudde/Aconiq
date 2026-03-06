import { Suspense } from "react";
import { Outlet } from "react-router";
import { AppShell } from "@/ui/app-shell";
import { PageSkeleton } from "@/ui/page-skeleton";
import { ErrorBoundary } from "@/ui/error-boundary";

export function RootLayout() {
  return (
    <AppShell>
      <ErrorBoundary>
        <Suspense fallback={<PageSkeleton />}>
          <Outlet />
        </Suspense>
      </ErrorBoundary>
    </AppShell>
  );
}

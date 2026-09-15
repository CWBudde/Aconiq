import { lazy } from "react";
import { createBrowserRouter, Navigate } from "react-router";
import type { RouteObject } from "react-router";

const MapPage = lazy(() => import("@/pages/map"));
const ImportPage = lazy(() => import("@/pages/import"));
const RunPage = lazy(() => import("@/pages/run"));
const ResultsPage = lazy(() => import("@/pages/results"));
const ExportPage = lazy(() => import("@/pages/export"));
const StatusPage = lazy(() => import("@/pages/status"));
const SettingsPage = lazy(() => import("@/pages/settings"));
const WelcomePage = lazy(() => import("@/pages/welcome"));
const NotFoundPage = lazy(() => import("@/pages/not-found"));

import { RootLayout } from "@/layouts/root-layout";

/**
 * The route table, exported apart from the router so `routes.test.tsx` can
 * mount it in memory. `createBrowserRouter` is the one browser binding.
 */
export const routes: RouteObject[] = [
  {
    element: <RootLayout />,
    children: [
      { index: true, element: <Navigate to="/welcome" replace /> },
      { path: "welcome", element: <WelcomePage /> },
      { path: "model", element: <MapPage /> },
      { path: "import", element: <ImportPage /> },
      { path: "run", element: <RunPage /> },
      // A bare index route is what "nothing selected" means, and it is what
      // the rail link targets. Auto-redirecting to a run would rewrite
      // history, fight Back, and have to guess before `useRuns` resolves.
      //
      // Both children hold the *same* element, not a parent/`<Outlet/>` split:
      // React Router applies no key to a rendered route, so one component
      // instance spans `/results`, `/results/a` and `/results/b`, and the
      // detail pane's open tab survives a selection change. Two separate
      // `lazy()` calls would produce two component types and remount.
      {
        path: "results",
        children: [
          { index: true, element: <ResultsPage /> },
          { path: ":runId", element: <ResultsPage /> },
        ],
      },
      // Same shape as `results` above, and for the same reasons.
      {
        path: "export",
        children: [
          { index: true, element: <ExportPage /> },
          { path: ":runId", element: <ExportPage /> },
        ],
      },
      { path: "status", element: <StatusPage /> },
      { path: "settings", element: <SettingsPage /> },
      // A child of the layout, so a mistyped URL keeps the rail, the header
      // heading and a way out. Retired paths land here deliberately: a
      // redirect entry would keep a missed migration working forever.
      { path: "*", element: <NotFoundPage /> },
    ],
  },
];

export const router = createBrowserRouter(routes, {
  basename: import.meta.env.BASE_URL,
});

import { lazy } from "react";
import { createBrowserRouter, Navigate } from "react-router";

const MapPage = lazy(() => import("@/pages/map"));
const ImportPage = lazy(() => import("@/pages/import"));
const RunPage = lazy(() => import("@/pages/run"));
const ResultsPage = lazy(() => import("@/pages/results"));
const ExportPage = lazy(() => import("@/pages/export"));
const StatusPage = lazy(() => import("@/pages/status"));
const SettingsPage = lazy(() => import("@/pages/settings"));
const WelcomePage = lazy(() => import("@/pages/welcome"));

import { RootLayout } from "@/layouts/root-layout";

export const router = createBrowserRouter(
  [
    {
      element: <RootLayout />,
      children: [
        { index: true, element: <Navigate to="/welcome" replace /> },
        { path: "welcome", element: <WelcomePage /> },
        { path: "map", element: <MapPage /> },
        { path: "import", element: <ImportPage /> },
        { path: "run", element: <RunPage /> },
        { path: "results", element: <ResultsPage /> },
        { path: "export", element: <ExportPage /> },
        { path: "status", element: <StatusPage /> },
        { path: "settings", element: <SettingsPage /> },
      ],
    },
  ],
  { basename: import.meta.env.BASE_URL },
);

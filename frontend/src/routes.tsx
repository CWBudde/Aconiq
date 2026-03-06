import { lazy } from "react";
import { createBrowserRouter, Navigate } from "react-router";

const MapPage = lazy(() => import("@/pages/map"));
const ImportPage = lazy(() => import("@/pages/import"));
const RunPage = lazy(() => import("@/pages/run"));
const ResultsPage = lazy(() => import("@/pages/results"));
const ExportPage = lazy(() => import("@/pages/export"));
const StatusPage = lazy(() => import("@/pages/status"));
const SettingsPage = lazy(() => import("@/pages/settings"));

import { RootLayout } from "@/layouts/root-layout";

export const router = createBrowserRouter([
  {
    element: <RootLayout />,
    children: [
      { index: true, element: <Navigate to="/map" replace /> },
      { path: "map", element: <MapPage /> },
      { path: "import", element: <ImportPage /> },
      { path: "run", element: <RunPage /> },
      { path: "results", element: <ResultsPage /> },
      { path: "export", element: <ExportPage /> },
      { path: "status", element: <StatusPage /> },
      { path: "settings", element: <SettingsPage /> },
    ],
  },
]);

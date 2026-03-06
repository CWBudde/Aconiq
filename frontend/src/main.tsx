import React from "react";
import { createRoot } from "react-dom/client";
import { App } from "./App";
import "./styles/globals.css";

// Dev-mode long-task telemetry.
// Logs interactions that block the main thread for more than 50 ms.
if (import.meta.env.DEV && typeof PerformanceObserver !== "undefined") {
  try {
    const observer = new PerformanceObserver((list) => {
      for (const entry of list.getEntries()) {
        console.warn(
          `[perf] long task ${entry.duration.toFixed(1)} ms at ${entry.startTime.toFixed(0)} ms`,
          entry,
        );
      }
    });
    observer.observe({ type: "longtasks", buffered: true });
  } catch {
    // PerformanceObserver may not support longtasks in all environments.
  }
}

const rootElement = document.getElementById("root");
if (!rootElement) {
  throw new Error("Missing root element");
}

createRoot(rootElement).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
);

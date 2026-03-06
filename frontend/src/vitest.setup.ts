import "@testing-library/jest-dom/vitest";
import { afterEach } from "vitest";
import { cleanup } from "@testing-library/react";

// Ensure RTL cleans up the DOM after every test.
afterEach(cleanup);

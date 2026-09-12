import "@testing-library/jest-dom/vitest";
// jsdom has no IndexedDB; browser-mode persistence needs one in every test
// file that touches the backend, so it is installed globally here.
import "fake-indexeddb/auto";
import { afterEach, beforeEach, vi } from "vitest";
import { cleanup } from "@testing-library/react";

// Ensure RTL cleans up the DOM after every test.
afterEach(cleanup);

beforeEach(() => {
  if (typeof window.matchMedia !== "function") {
    window.matchMedia = vi.fn().mockImplementation((query: string) => ({
      matches: false,
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
  }
});

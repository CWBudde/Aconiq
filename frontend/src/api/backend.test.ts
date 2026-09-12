import { describe, expect, it } from "vitest";
import type { Backend } from "./backend";
import { browserBackend } from "./browser-backend";
import { httpBackend } from "./http-backend";

/**
 * Both implementations must be interchangeable behind `Backend`: the pages
 * and hooks only ever see the interface, so a method one implementation
 * lacks would fail at runtime in exactly one mode. The `satisfies` lines make
 * that a compile error instead.
 */
browserBackend satisfies Backend;
httpBackend satisfies Backend;

describe("backend capabilities", () => {
  it("name the mode they belong to", () => {
    expect(browserBackend.capabilities.kind).toBe("browser");
    expect(httpBackend.capabilities.kind).toBe("http");
  });

  it("only the browser can generate an export bundle itself", () => {
    expect(browserBackend.capabilities.canExport).toBe(true);
    expect(httpBackend.capabilities.canExport).toBe(false);
  });

  it("only the API runs against the model saved in the project", () => {
    expect(browserBackend.capabilities.runsAgainstSavedModel).toBe(false);
    expect(httpBackend.capabilities.runsAgainstSavedModel).toBe(true);
  });
});

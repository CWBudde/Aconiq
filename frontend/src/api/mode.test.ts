import { beforeEach, describe, expect, it } from "vitest";
import {
  API_BASE_URL_OVERRIDE_KEY,
  apiURL,
  clearAPIBaseURLOverride,
  getAPIBaseURL,
  hasAPIBaseURLOverride,
  setAPIBaseURLOverride,
} from "./mode";

describe("api mode helpers", () => {
  beforeEach(() => {
    localStorage.clear();
  });

  it("normalizes and applies a browser override", () => {
    setAPIBaseURLOverride(" https://example.com/ ");

    expect(localStorage.getItem(API_BASE_URL_OVERRIDE_KEY)).toBe(
      "https://example.com",
    );
    expect(hasAPIBaseURLOverride()).toBe(true);
    expect(getAPIBaseURL()).toBe("https://example.com");
    expect(apiURL("/api/v1/runs")).toBe("https://example.com/api/v1/runs");
  });

  it("removes the override when cleared or set to blank", () => {
    setAPIBaseURLOverride("https://example.com");
    clearAPIBaseURLOverride();

    expect(localStorage.getItem(API_BASE_URL_OVERRIDE_KEY)).toBeNull();
    expect(hasAPIBaseURLOverride()).toBe(false);

    setAPIBaseURLOverride("   ");

    expect(localStorage.getItem(API_BASE_URL_OVERRIDE_KEY)).toBeNull();
    expect(hasAPIBaseURLOverride()).toBe(false);
  });
});

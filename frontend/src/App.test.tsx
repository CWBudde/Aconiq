import { expect, test } from "vitest";
import { App } from "./App";

test("App component is defined", () => {
  expect(App).toBeDefined();
});

test("query keys are structured correctly", async () => {
  const { queryKeys } = await import("./api/query-keys");
  expect(queryKeys.health.all).toEqual(["health"]);
  expect(queryKeys.project.status()).toEqual(["project", "status"]);
});

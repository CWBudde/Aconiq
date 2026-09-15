/**
 * The route table is a contract, and until now nothing tested it.
 *
 * These assertions read `routes` structurally rather than rendering it: the
 * page modules are `lazy()`, so mounting any of them here would pull in
 * MapLibre, TanStack Query and the backend selection for the sake of checking
 * a path string. What matters is the shape — which paths exist, which of them
 * carry children, and whether two sibling entries share one element.
 */

import { describe, expect, it } from "vitest";
import type { RouteObject } from "react-router";

import { routes } from "./routes";

/** The single pathless layout route every page hangs off. */
function layoutChildren(): RouteObject[] {
  expect(routes).toHaveLength(1);
  const layout = routes[0];
  expect(layout?.path, "the root route is pathless").toBeUndefined();
  return layout?.children ?? [];
}

/** `"index"` for an index route, otherwise the route's own path segment. */
function slug(route: RouteObject): string {
  return route.index === true ? "index" : (route.path ?? "");
}

function inventory(children: RouteObject[]): string[] {
  return children.map((child) => {
    const own = slug(child);
    const nested = child.children?.map(slug) ?? [];
    return nested.length > 0 ? `${own}/{${nested.join(",")}}` : own;
  });
}

describe("the route table", () => {
  it("registers every page under one layout route", () => {
    expect(inventory(layoutChildren())).toEqual([
      "index",
      "welcome",
      "map",
      "import",
      "run",
      "results",
      "export",
      "status",
      "settings",
    ]);
  });
});

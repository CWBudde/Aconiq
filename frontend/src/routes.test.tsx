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
      "results/{index,:runId}",
      "export",
      "status",
      "settings",
    ]);
  });

  it("gives a section's index and :runId children one element type", () => {
    // The no-remount property depends on both children holding the *same*
    // lazy binding. Two `lazy(() => import("@/pages/results"))` calls give two
    // component types, and every selection change would silently remount the
    // detail pane — resetting its open tab, with nothing in the suite to say so.
    for (const section of ["results"]) {
      const parent = layoutChildren().find((c) => c.path === section);
      const children = parent?.children ?? [];
      expect(children, section).toHaveLength(2);
      const [index, param] = children;
      expect(index?.index, section).toBe(true);
      expect(param?.path, section).toBe(":runId");
      expect((index?.element as { type: unknown }).type).toBe(
        (param?.element as { type: unknown }).type,
      );
    }
  });
});

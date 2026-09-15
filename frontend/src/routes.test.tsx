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
import { matchRoutes } from "react-router";
import type { RouteObject } from "react-router";

import { routes } from "./routes";

/**
 * The path of the deepest route a location matches, or null for no match.
 *
 * The cast works around a react-router typing gap: its public `RouteObject`
 * is not assignable to the `AgnosticRouteObject` its own `matchRoutes`
 * declares, under `exactOptionalPropertyTypes`. The value is the very table
 * `createBrowserRouter` is handed in `routes.tsx`.
 */
function matchedPath(pathname: string): string | null {
  const matches = matchRoutes(
    routes as Parameters<typeof matchRoutes>[0],
    pathname,
  );
  const last = matches?.[matches.length - 1];
  return last?.route.path ?? (last?.route.index === true ? "index" : null);
}

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
      "model",
      "import",
      "run",
      "results/{index,:runId}",
      "export/{index,:runId}",
      "status",
      "settings",
      "*",
    ]);
  });

  it("gives a section's index and :runId children one element type", () => {
    // The no-remount property depends on both children holding the *same*
    // lazy binding. Two `lazy(() => import("@/pages/results"))` calls give two
    // component types, and every selection change would silently remount the
    // detail pane — resetting its open tab, with nothing in the suite to say so.
    for (const section of ["results", "export"]) {
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

  it("catches a path no route claims", () => {
    // `:runId` swallows `/results/typo` — that is the unknown-run state, not a
    // 404 — but a third segment is nobody's.
    expect(matchedPath("/nonsense")).toBe("*");
    expect(matchedPath("/results/a/b")).toBe("*");
  });

  it("does not keep the retired paths alive", () => {
    // `/map` was renamed, and there is deliberately no redirect entry for it:
    // a redirect would keep a `<Link>` or a spec that was missed in the rename
    // working silently and forever. The break is meant to be loud.
    expect(matchedPath("/map")).toBe("*");
  });
});

import { createElement, type ReactNode } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ModelSaveResult, RunSpec } from "./backend";
import { buildCreateRunRequest } from "./http-backend";
import { useIsSavingModel, useSaveModel } from "./hooks";
import { queryClient } from "./query-client";

const saveState = vi.hoisted(() => ({
  respond: (): Promise<ModelSaveResult> =>
    Promise.resolve({ featureCount: 0, warnings: [] }),
}));

vi.mock("./backend", () => ({
  backend: {
    capabilities: {
      kind: "http",
      canExport: false,
      runsAgainstSavedModel: true,
      runsChangeExternally: true,
    },
    saveModel: () => saveState.respond(),
    getProjectStatus: () => Promise.resolve(null),
  },
}));

const baseSpec: RunSpec = {
  standardId: "rls19-road",
  version: "2019",
  profile: "default",
  params: {},
  receiverMode: "auto-grid",
};

/**
 * The request body is the contract the API gates on: a scaffold-tier standard
 * is refused unless `experimental` is present, and a standard that needs no
 * acknowledgement must not carry one.
 */
describe("buildCreateRunRequest", () => {
  it("carries the acknowledgement for a scaffold-tier run", () => {
    const request = buildCreateRunRequest({
      ...baseSpec,
      standardId: "cnossos-road",
      experimental: true,
    });

    expect(request.experimental).toBe(true);
    expect(request.standard_id).toBe("cnossos-road");
  });

  it("omits the field entirely when nothing was acknowledged", () => {
    const request = buildCreateRunRequest(baseSpec);

    expect(request).not.toHaveProperty("experimental");
    expect(Object.keys(request)).not.toContain("experimental");
  });

  it("omits the field rather than sending false", () => {
    const request = buildCreateRunRequest({
      ...baseSpec,
      experimental: false,
    });

    expect(request).not.toHaveProperty("experimental");
  });

  it("maps the rest of the spec onto the API field names", () => {
    const request = buildCreateRunRequest({
      ...baseSpec,
      profile: "strict",
      params: { temperature_c: "10" },
      receiverMode: "custom",
    });

    expect(request).toMatchObject({
      standard_id: "rls19-road",
      standard_version: "2019",
      standard_profile: "strict",
      receiver_mode: "custom",
      params: { temperature_c: "10" },
    });
  });
});

/**
 * `useIsSavingModel` reads the mutation cache by key rather than the state of
 * one `useSaveModel` instance, so every surface sees a save any surface
 * started. This runs both under the app's own query client, the way the
 * app does.
 */
describe("useIsSavingModel", () => {
  function wrapper({ children }: { children: ReactNode }) {
    return createElement(
      QueryClientProvider,
      { client: queryClient },
      children,
    );
  }

  beforeEach(() => {
    queryClient.clear();
  });

  it("is true while a save is pending and false after it settles", async () => {
    let resolve: (() => void) | undefined;
    saveState.respond = () =>
      new Promise((r) => {
        resolve = () => {
          r({ featureCount: 1, warnings: [] });
        };
      });

    const save = renderHook(() => useSaveModel(), { wrapper });
    const observer = renderHook(() => useIsSavingModel(), { wrapper });
    expect(observer.result.current).toBe(false);

    let done: Promise<ModelSaveResult> | undefined;
    act(() => {
      done = save.result.current.mutateAsync({
        model: { type: "FeatureCollection", features: [] },
      });
    });
    await waitFor(() => {
      expect(observer.result.current).toBe(true);
    });

    await act(async () => {
      resolve?.();
      await done;
    });
    await waitFor(() => {
      expect(observer.result.current).toBe(false);
    });
  });
});

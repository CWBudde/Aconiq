import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import { useState } from "react";
import { APIRequestError } from "@/api/api-error";
import type { LglnImportRequest, LglnImportResponse } from "@/api/client";
import { useModelStore } from "@/model/model-store";
import type { ModelFeature } from "@/model/types";
import ImportPage from "@/pages/import";
import { m } from "@/i18n/messages";
import type { BBoxText } from "./bbox";
import { LglnImport } from "./lgln-import";
import { estimateLglnTiles, lglnFailureText } from "./lgln";

const backendState = vi.hoisted(() => ({
  canImportLGLN: true,
  importFromLGLN: vi.fn<(req: LglnImportRequest) => Promise<unknown>>(),
}));

vi.mock("@/api/backend", () => ({
  backend: {
    get capabilities() {
      return {
        kind: backendState.canImportLGLN ? "http" : "browser",
        canImportLGLN: backendState.canImportLGLN,
      };
    },
    importFromLGLN: (req: LglnImportRequest) =>
      backendState.importFromLGLN(req),
  },
}));

function square(x: number, y: number): number[][][] {
  const d = 0.00005;
  return [
    [
      [x - d, y - d],
      [x + d, y - d],
      [x + d, y + d],
      [x - d, y + d],
      [x - d, y - d],
    ],
  ];
}

const response: LglnImportResponse = {
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      id: "DENILD0100000001",
      properties: {
        kind: "building",
        height_m: 11.4,
        import_format: "lgln-lod2",
      },
      geometry: { type: "Polygon", coordinates: square(9.735, 52.375) },
    },
  ],
  tiles: [{ id: "32_550_5803", date: "2024-05-01" }],
  attribution: "© LGLN 2024, dl-de/by-2-0",
  skipped: {},
};

const osmInside: ModelFeature = {
  id: "osm-way-1",
  kind: "building",
  heightM: 9,
  properties: { osm_id: "1" },
  geometry: {
    type: "Polygon",
    coordinates: square(
      9.736,
      52.376,
    ) as ModelFeature["geometry"]["coordinates"],
  },
};

function queryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
}

function Harness({
  initial,
  onCollection = () => undefined,
  onError = () => undefined,
}: {
  initial: BBoxText;
  onCollection?: (c: LglnImportResponse, bbox: unknown) => void;
  onError?: (message: string | null) => void;
}) {
  const [bbox, setBBox] = useState(initial);
  return (
    <LglnImport
      bbox={bbox}
      onBBoxChange={setBBox}
      onCollection={onCollection}
      onError={onError}
    />
  );
}

function renderPanel(props: Parameters<typeof Harness>[0]) {
  return render(
    <QueryClientProvider client={queryClient()}>
      <Harness {...props} />
    </QueryClientProvider>,
  );
}

const box: BBoxText = {
  south: "52.37",
  west: "9.73",
  north: "52.38",
  east: "9.74",
};

beforeEach(() => {
  backendState.canImportLGLN = true;
  backendState.importFromLGLN.mockReset();
  useModelStore.getState().reset();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("estimateLglnTiles", () => {
  it("counts about one tile for a box well under a kilometre", () => {
    // ~340 m × 330 m at 52°N.
    expect(
      estimateLglnTiles({
        south: 52.37,
        west: 9.73,
        north: 52.373,
        east: 9.735,
      }),
    ).toBe(2);
  });

  it("grows with the box and answers null for an inside-out one", () => {
    const large = estimateLglnTiles({
      south: 52.3,
      west: 9.7,
      north: 52.35,
      east: 9.8,
    });
    expect(large).toBeGreaterThan(9);
    expect(
      estimateLglnTiles({ south: 52.4, west: 9.7, north: 52.3, east: 9.8 }),
    ).toBeNull();
  });
});

describe("lglnFailureText", () => {
  it("explains a box that covers too many tiles", () => {
    expect(
      lglnFailureText(
        new APIRequestError({
          code: "lgln_too_many_tiles",
          message: "bbox covers 12 tiles",
        }),
      ),
    ).toBe(m.error_lgln_too_many_tiles());
  });

  it("explains an unreachable download service", () => {
    expect(
      lglnFailureText(
        new APIRequestError({ code: "lgln_unavailable", message: "502" }),
      ),
    ).toBe(m.error_lgln_unavailable());
  });

  it("keeps the envelope's message and hint for anything else", () => {
    expect(
      lglnFailureText(
        new APIRequestError({
          code: "bad_request",
          message: "bbox invalid",
          hint: "south must be below north",
        }),
      ),
    ).toBe("bbox invalid — south must be below north");
  });
});

describe("LglnImport", () => {
  it("renders the box, the tile estimate and the load button", () => {
    renderPanel({ initial: box });

    expect(screen.getByText(m.heading_import_from_lgln())).toBeInTheDocument();
    expect(screen.getByLabelText<HTMLInputElement>(m.label_south()).value).toBe(
      "52.37",
    );
    expect(screen.getByText(/ca\. \d+/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: m.action_fetch_from_lgln() }),
    ).toBeEnabled();
  });

  it("is disabled in browser mode and says it needs the local server", () => {
    backendState.canImportLGLN = false;
    renderPanel({ initial: box });

    expect(screen.getByText(m.msg_lgln_needs_server())).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: m.action_fetch_from_lgln() }),
    ).toBeDisabled();
    expect(screen.getByLabelText(m.label_south())).toBeDisabled();
  });

  it("calls the API with the box and hands the collection up", async () => {
    backendState.importFromLGLN.mockResolvedValue(response);
    const onCollection = vi.fn();
    renderPanel({ initial: box, onCollection });

    fireEvent.click(
      screen.getByRole("button", { name: m.action_fetch_from_lgln() }),
    );

    await waitFor(() => {
      expect(onCollection).toHaveBeenCalledTimes(1);
    });
    expect(backendState.importFromLGLN).toHaveBeenCalledWith({
      south: 52.37,
      west: 9.73,
      north: 52.38,
      east: 9.74,
    });
    expect(onCollection).toHaveBeenCalledWith(response, {
      south: 52.37,
      west: 9.73,
      north: 52.38,
      east: 9.74,
    });
  });

  it("reports a box that covers too many tiles", async () => {
    backendState.importFromLGLN.mockRejectedValue(
      new APIRequestError({
        code: "lgln_too_many_tiles",
        message: "bbox covers 12 tiles",
      }),
    );
    const onError = vi.fn();
    renderPanel({ initial: box, onError });

    fireEvent.click(
      screen.getByRole("button", { name: m.action_fetch_from_lgln() }),
    );

    await waitFor(() => {
      expect(onError).toHaveBeenLastCalledWith(m.error_lgln_too_many_tiles());
    });
  });

  it("refuses to load without a complete box", () => {
    const onError = vi.fn();
    renderPanel({ initial: { ...box, east: "" }, onError });

    fireEvent.click(
      screen.getByRole("button", { name: m.action_fetch_from_lgln() }),
    );

    expect(onError).toHaveBeenLastCalledWith(m.msg_bbox_required());
    expect(backendState.importFromLGLN).not.toHaveBeenCalled();
  });

  it("fills the box from the model's extent", () => {
    useModelStore.getState().loadModel({
      features: [osmInside],
      receivers: [],
      calcArea: null,
    });
    renderPanel({ initial: { south: "", west: "", north: "", east: "" } });

    fireEvent.click(
      screen.getByRole("button", { name: m.action_use_model_extent() }),
    );

    expect(screen.getByLabelText<HTMLInputElement>(m.label_west()).value).toBe(
      "9.735950",
    );
  });
});

describe("ImportPage with an LGLN load", () => {
  function renderPage() {
    const router = createMemoryRouter(
      [
        { path: "/import", element: <ImportPage /> },
        { path: "/model", element: <div>Map page</div> },
      ],
      { initialEntries: ["/import"] },
    );
    return render(
      <QueryClientProvider client={queryClient()}>
        <RouterProvider router={router} />
      </QueryClientProvider>,
    );
  }

  async function openTab(name: string): Promise<void> {
    await userEvent.setup().click(screen.getByRole("tab", { name }));
  }

  it("shares the box with the OSM tab", async () => {
    renderPage();
    await openTab(m.action_import_from_osm());
    fireEvent.change(screen.getByLabelText(m.label_south()), {
      target: { value: "52.37" },
    });

    await openTab(m.action_import_from_lgln());

    expect(screen.getByLabelText<HTMLInputElement>(m.label_south()).value).toBe(
      "52.37",
    );
  });

  it("previews how many OSM buildings are replaced, then replaces them in one undo step", async () => {
    useModelStore.getState().loadModel({
      features: [osmInside],
      receivers: [],
      calcArea: null,
    });
    backendState.importFromLGLN.mockResolvedValue(response);
    renderPage();
    await openTab(m.action_import_from_lgln());
    for (const [label, value] of [
      [m.label_south(), box.south],
      [m.label_west(), box.west],
      [m.label_north(), box.north],
      [m.label_east(), box.east],
    ] as const) {
      fireEvent.change(screen.getByLabelText(label), { target: { value } });
    }

    fireEvent.click(
      screen.getByRole("button", { name: m.action_fetch_from_lgln() }),
    );

    await waitFor(() => screen.getByText(m.heading_import_preview()));
    expect(
      screen.getByText(m.msg_lgln_replaces_osm({ count: 1 })),
    ).toBeInTheDocument();
    expect(screen.getByText(m.msg_lgln_adds({ count: 1 }))).toBeInTheDocument();
    expect(screen.getByText(/dl-de\/by-2-0/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: m.action_import_replace() }),
    ).toBeNull();

    fireEvent.click(
      screen.getByRole("button", { name: m.action_import_lgln_apply() }),
    );

    await waitFor(() => screen.getByText(m.status_import_complete()));
    expect(useModelStore.getState().features.map((f) => f.id)).toEqual([
      "DENILD0100000001",
    ]);

    useModelStore.getState().undo();
    expect(useModelStore.getState().features.map((f) => f.id)).toEqual([
      "osm-way-1",
    ]);
  });
});

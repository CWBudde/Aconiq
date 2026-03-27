import { beforeEach, describe, expect, it } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { createMemoryRouter, RouterProvider } from "react-router";
import ImportPage from "./import";
import { useModelStore } from "@/model/model-store";

const validGeoJSON = JSON.stringify({
  type: "FeatureCollection",
  features: [
    {
      type: "Feature",
      properties: { kind: "source", source_type: "point" },
      geometry: { type: "Point", coordinates: [10, 51] },
    },
    {
      type: "Feature",
      properties: { kind: "building", height_m: 10 },
      geometry: {
        type: "Polygon",
        coordinates: [
          [
            [0, 0],
            [1, 0],
            [1, 1],
            [0, 1],
            [0, 0],
          ],
        ],
      },
    },
  ],
});

function makeFile(content: string, name = "model.geojson"): File {
  return new File([content], name, { type: "application/json" });
}

function getFileInput(): HTMLInputElement {
  const input = document.querySelector<HTMLInputElement>('input[type="file"]');
  if (!input) {
    throw new Error("file input not found");
  }
  return input;
}

function renderImportPage() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
      },
      mutations: {
        retry: false,
      },
    },
  });
  const router = createMemoryRouter(
    [
      { path: "/import", element: <ImportPage /> },
      { path: "/map", element: <div>Map page</div> },
    ],
    { initialEntries: ["/import"] },
  );
  return render(
    <QueryClientProvider client={queryClient}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  useModelStore.getState().reset();
});

describe("ImportPage", () => {
  it("renders the upload step initially", () => {
    renderImportPage();
    expect(screen.getByText("Import GeoJSON")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /choose file/i }),
    ).toBeInTheDocument();
  });

  it("shows preview after a valid file is selected", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => {
      expect(screen.getByText("Import Preview")).toBeInTheDocument();
    });
    expect(screen.getByText(/2 features normalized/i)).toBeInTheDocument();
  });

  it("shows an error for invalid JSON", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile("not json")] } });
    await waitFor(() => {
      expect(screen.getByText(/failed to parse/i)).toBeInTheDocument();
    });
  });

  it("shows an error for a non-FeatureCollection JSON", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, {
      target: { files: [makeFile('{"type":"Feature"}')] },
    });
    await waitFor(() => {
      expect(
        screen.getByText(/must be a GeoJSON FeatureCollection/i),
      ).toBeInTheDocument();
    });
  });

  it("loads features into the model store on confirm", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    fireEvent.click(screen.getByRole("button", { name: /import/i }));
    expect(useModelStore.getState().features).toHaveLength(2);
  });

  it("shows done step after confirm", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    fireEvent.click(screen.getByRole("button", { name: /import/i }));
    expect(screen.getByText("Import Complete")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /go to map/i }),
    ).toBeInTheDocument();
  });

  it("goes back to upload step from preview", async () => {
    renderImportPage();
    const input = getFileInput();
    fireEvent.change(input, { target: { files: [makeFile(validGeoJSON)] } });
    await waitFor(() => screen.getByText("Import Preview"));
    fireEvent.click(screen.getByRole("button", { name: /back/i }));
    expect(screen.getByText("Import GeoJSON")).toBeInTheDocument();
  });
});

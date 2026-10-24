import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { m } from "@/i18n/messages";
import { PhotoCorners } from "./photo-corners";

/** jsdom loads no image, so the photo's size arrives as the browser's would. */
function loadPhoto(w: number, h: number) {
  const img = screen.getByAltText(m.label_photo_file());
  Object.defineProperty(img, "naturalWidth", { value: w });
  Object.defineProperty(img, "naturalHeight", { value: h });
  fireEvent.load(img);
}

function typeCorner(corner: string, x: number, y: number) {
  fireEvent.change(screen.getByLabelText(m.label_photo_corner_x({ corner })), {
    target: { value: String(x) },
  });
  fireEvent.change(screen.getByLabelText(m.label_photo_corner_y({ corner })), {
    target: { value: String(y) },
  });
}

const rectify = () =>
  screen.getByRole("button", { name: m.action_rectify_photo() });

describe("PhotoCorners", () => {
  it("asks for the corners in order, and places a click at the photo's pixel", () => {
    render(
      <PhotoCorners
        url="blob:photo"
        busy={false}
        error={null}
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    loadPhoto(2000, 1000);

    expect(
      screen.getByText(
        m.msg_photo_corner_prompt({ corner: m.label_corner_bottom_left() }),
      ),
    ).toBeTruthy();

    // The photo is shown 500 CSS px wide: a click 100 px in is pixel 400.
    const svg = screen.getByRole("group", { name: m.heading_photo_corners() });
    svg.getBoundingClientRect = () =>
      ({ left: 0, top: 0, width: 500, height: 250 }) as DOMRect;
    fireEvent.pointerDown(svg, { button: 0, clientX: 100, clientY: 200 });

    const x = screen.getByLabelText<HTMLInputElement>(
      m.label_photo_corner_x({ corner: m.label_corner_bottom_left() }),
    );
    const y = screen.getByLabelText<HTMLInputElement>(
      m.label_photo_corner_y({ corner: m.label_corner_bottom_left() }),
    );
    expect([x.value, y.value]).toEqual(["400", "800"]);
    expect(
      screen.getByText(
        m.msg_photo_corner_prompt({ corner: m.label_corner_bottom_right() }),
      ),
    ).toBeTruthy();
  });

  it("nudges a corner with the arrow keys, ten pixels with Shift", () => {
    render(
      <PhotoCorners
        url="blob:photo"
        busy={false}
        error={null}
        onSubmit={vi.fn()}
        onCancel={vi.fn()}
      />,
    );
    loadPhoto(2000, 1000);
    typeCorner(m.label_corner_bottom_left(), 100, 900);

    const marker = screen.getByRole("button", {
      name: m.label_photo_corner({ corner: m.label_corner_bottom_left() }),
    });
    fireEvent.keyDown(marker, { key: "ArrowRight", shiftKey: true });
    fireEvent.keyDown(marker, { key: "ArrowUp" });

    expect(marker.getAttribute("cx")).toBe("110");
    expect(marker.getAttribute("cy")).toBe("899");
  });

  it("hands over the four corners and the record once there is a reference", () => {
    const onSubmit = vi.fn();
    render(
      <PhotoCorners
        url="blob:photo"
        busy={false}
        error={null}
        onSubmit={onSubmit}
        onCancel={vi.fn()}
      />,
    );
    typeCorner(m.label_corner_bottom_left(), 100, 900);
    typeCorner(m.label_corner_bottom_right(), 865, 846);
    typeCorner(m.label_corner_top_right(), 885, 115);
    typeCorner(m.label_corner_top_left(), 123, 139);
    expect(rectify()).toHaveProperty("disabled", true);

    fireEvent.change(screen.getByLabelText(m.label_photo_reference()), {
      target: { value: " store/0042 " },
    });
    fireEvent.change(screen.getByLabelText(m.label_photo_photographer()), {
      target: { value: "C. B." },
    });
    fireEvent.click(rectify());

    expect(onSubmit).toHaveBeenCalledWith(
      [
        [100, 900],
        [865, 846],
        [885, 115],
        [123, 139],
      ],
      { sourceId: "store/0042", attribution: "C. B.", capturedAt: "" },
    );
  });
});

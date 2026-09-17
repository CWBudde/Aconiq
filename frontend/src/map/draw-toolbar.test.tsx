import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TooltipProvider } from "@/ui/components/tooltip";
import { DrawToolbar } from "./draw-toolbar";
import type { DrawMode } from "./use-draw";
import { m } from "@/i18n/messages";

/**
 * The toolbar is presentational — `pages/map.tsx` binds it to the draw
 * provider — so it needs no map and no WebGL. What it owns is the gate: when
 * the model is in a CRS the map cannot draw in, every tool is refused and the
 * tooltip has to say why. `pages/map.test.tsx` mocks this component out, so
 * that gate has had no test at all.
 */

const tools: { mode: DrawMode; label: () => string }[] = [
  { mode: "select", label: m.tool_select_edit },
  { mode: "point", label: m.tool_draw_point },
  { mode: "linestring", label: m.tool_draw_line },
  { mode: "polygon", label: m.tool_draw_polygon },
  { mode: "calc-area", label: m.tool_draw_calc_area },
];

interface ToolbarOptions {
  activeMode?: DrawMode;
  disabled?: boolean;
  disabledReason?: string;
}

function renderToolbar({
  activeMode = "static",
  disabled = false,
  disabledReason,
}: ToolbarOptions = {}) {
  const onModeChange = vi.fn<(mode: DrawMode) => void>();
  const onCancel = vi.fn();
  const onCoordinateEntry = vi.fn();
  render(
    <TooltipProvider delayDuration={0}>
      <DrawToolbar
        activeMode={activeMode}
        onModeChange={onModeChange}
        onCancel={onCancel}
        onCoordinateEntry={onCoordinateEntry}
        disabled={disabled}
        {...(disabledReason === undefined ? {} : { disabledReason })}
      />
    </TooltipProvider>,
  );
  return { onModeChange, onCancel, onCoordinateEntry };
}

function button(name: string): HTMLElement {
  return screen.getByRole("button", { name });
}

/**
 * Hovered with `userEvent`, deliberately, rather than a `fireEvent.pointerMove`
 * at the trigger.
 *
 * `fireEvent` dispatches straight at the node, so it will open a tooltip over a
 * control no user could reach, and report a pass for it. That is why the probe
 * matters even where, as here, it is not the assertion that bites.
 *
 * **But hovering is not what catches that regression here, and it is worth
 * being exact about why.** `userEvent` does honour `pointer-events: none` —
 * except the rule that would set it is Tailwind's `disabled:pointer-events-none`
 * from `ui/components/button.tsx`, and jsdom computes no stylesheet, so the
 * property is never actually there in this environment. A hover would pass over
 * a real `disabled` button too. What fails if the refused state regresses is
 * the pair of assertions below: `aria-disabled` being present, and Tab reaching
 * the button at all. Both were verified to fail against the `disabled` version.
 */
async function tooltipFor(name: string): Promise<string | null> {
  await userEvent.hover(button(name));
  const tooltip = await screen.findByRole("tooltip");
  return tooltip.textContent;
}

describe("DrawToolbar tools", () => {
  it("is a named toolbar, so its icon buttons are not five anonymous ones", () => {
    renderToolbar();
    expect(
      screen.getByRole("toolbar", { name: m.label_draw_tools() }),
    ).toBeInTheDocument();
  });

  it.each(tools)(
    "reports $mode to the caller when clicked",
    ({ mode, label }) => {
      const { onModeChange } = renderToolbar();

      fireEvent.click(button(label()));

      expect(onModeChange).toHaveBeenCalledWith(mode);
    },
  );

  it.each(tools)(
    "presses only the $mode button while it is active",
    ({ mode, label }) => {
      // `aria-pressed` is the only thing that tells a screen-reader user which
      // tool is armed; the sighted cue is the button variant alone.
      renderToolbar({ activeMode: mode });

      for (const tool of tools) {
        expect(button(tool.label())).toHaveAttribute(
          "aria-pressed",
          String(tool.mode === mode),
        );
      }
      expect(button(label())).toHaveAttribute("aria-pressed", "true");
    },
  );

  it("presses nothing in the static mode the toolbar rests in", () => {
    renderToolbar({ activeMode: "static" });
    for (const tool of tools) {
      expect(button(tool.label())).toHaveAttribute("aria-pressed", "false");
    }
  });
});

describe("DrawToolbar cancel", () => {
  it("offers no cancel while nothing is being drawn", () => {
    // Cancelling "static" is meaningless, and a permanently visible red X on
    // the toolbar reads as a destructive action on the model.
    renderToolbar({ activeMode: "static" });
    expect(
      screen.queryByRole("button", { name: m.action_cancel_drawing() }),
    ).not.toBeInTheDocument();
  });

  it.each(tools)("offers cancel while $mode is armed", ({ mode }) => {
    renderToolbar({ activeMode: mode });
    expect(button(m.action_cancel_drawing())).toBeInTheDocument();
  });

  it("calls back rather than changing mode itself", () => {
    // Abandoning a drawing has to go through terra-draw to drop the partial
    // geometry; setting the mode to "static" here would leave it on the map.
    const { onCancel, onModeChange } = renderToolbar({ activeMode: "polygon" });

    fireEvent.click(button(m.action_cancel_drawing()));

    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onModeChange).not.toHaveBeenCalled();
  });
});

describe("DrawToolbar when drawing is refused", () => {
  it("marks every tool refused without making it unreachable", () => {
    // terra-draw emits WGS84. Over a metric model every one of these would
    // enter coordinates in the wrong CRS, the calculation area included.
    //
    // `aria-disabled` rather than `disabled`, and the assertion says both
    // halves: a real `disabled` attribute would announce the same thing and
    // then take the button out of the pointer and focus order, which is what
    // kills the tooltip that has to explain it.
    renderToolbar({ disabled: true, disabledReason: "Model is in EPSG:25832" });

    for (const tool of tools) {
      expect(button(tool.label())).toHaveAttribute("aria-disabled", "true");
      expect(button(tool.label())).not.toBeDisabled();
    }
  });

  it("accepts no clicks while refused", async () => {
    const { onModeChange } = renderToolbar({
      disabled: true,
      disabledReason: "Model is in EPSG:25832",
    });

    // A real click, not a dispatched one: `aria-disabled` does not stop
    // activation by itself, so the handler has to swallow it.
    await userEvent.click(button(m.tool_draw_point()));

    expect(onModeChange).not.toHaveBeenCalled();
  });

  it("is reachable by keyboard, so the reason is not mouse-only", async () => {
    renderToolbar({ disabled: true, disabledReason: "Model is in EPSG:25832" });

    await userEvent.tab();

    // A `disabled` button is skipped by Tab entirely; this is the assertion
    // that would catch a regression to one.
    expect(button(m.tool_select_edit())).toHaveFocus();
  });

  it("puts the reason in the tooltip in place of the tool's name", async () => {
    // The decision the component documents: the tooltip is the only surface
    // that can explain a dead button, and "Draw Point" said over a dead button
    // explains nothing. A regression to "Draw Point — <reason>" fails here.
    const reason = "Model is in EPSG:25832";
    renderToolbar({ disabled: true, disabledReason: reason });

    expect(await tooltipFor(m.tool_draw_point())).toBe(reason);
  });

  it("explains the calculation-area tool too, which has its own tooltip", async () => {
    // It sits below the separator and is built separately from the four
    // geometry tools, so it has its own chance to miss the reason.
    const reason = "Model is in EPSG:25832";
    renderToolbar({ disabled: true, disabledReason: reason });

    expect(await tooltipFor(m.tool_draw_calc_area())).toBe(reason);
  });

  it("falls back to the tool's name when no reason was supplied", async () => {
    // A caller that disables without saying why should still get a labelled
    // tooltip rather than an empty one.
    renderToolbar({ disabled: true });

    expect(await tooltipFor(m.tool_draw_point())).toBe(m.tool_draw_point());
  });

  it("names the tool while it is usable", async () => {
    // The reason must not leak into the enabled state: `disabledReason` is
    // passed unconditionally by `pages/map.tsx`.
    renderToolbar({
      disabled: false,
      disabledReason: "Model is in EPSG:25832",
    });

    expect(await tooltipFor(m.tool_draw_point())).toBe(m.tool_draw_point());
  });

  it("still lets a drawing already in progress be abandoned", () => {
    // The cancel control is deliberately outside the gate. The gate can come
    // on while a shape is half-drawn — the model's CRS is decided by what has
    // been imported — and a partial geometry that cannot be cancelled would
    // sit on the map with no way off it.
    const { onCancel } = renderToolbar({
      activeMode: "polygon",
      disabled: true,
      disabledReason: "Model is in EPSG:25832",
    });

    const cancel = button(m.action_cancel_drawing());
    expect(cancel).toBeEnabled();
    fireEvent.click(cancel);
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});

describe("DrawToolbar coordinate entry", () => {
  it("offers a labelled control that opens the typed path", async () => {
    const { onCoordinateEntry } = renderToolbar();

    await userEvent.click(button(m.action_enter_coordinates()));

    expect(onCoordinateEntry).toHaveBeenCalledTimes(1);
  });

  it("stays usable in exactly the state every drawing tool is refused in", async () => {
    // The point of this control. `disabled` means terra-draw's WGS 84 output
    // could not be moved into the model's CRS; typed numbers are already in
    // it, so there is nothing to project and nothing to refuse. On a model in
    // a CRS the kernel cannot handle, this is the only way to add a feature.
    const { onCoordinateEntry } = renderToolbar({
      disabled: true,
      disabledReason: "Model is in EPSG:3035",
    });

    const control = button(m.action_enter_coordinates());
    expect(control).not.toHaveAttribute("aria-disabled");
    expect(control).toBeEnabled();

    await userEvent.click(control);

    expect(onCoordinateEntry).toHaveBeenCalledTimes(1);
  });

  it("is reachable by Tab, which is the whole point of it", async () => {
    renderToolbar();

    // Five drawing tools come first in the toolbar, then this one.
    for (let i = 0; i < 6; i++) await userEvent.tab();

    expect(button(m.action_enter_coordinates())).toHaveFocus();
  });

  it("names itself in its tooltip rather than repeating the draw reason", async () => {
    renderToolbar({ disabled: true, disabledReason: "Model is in EPSG:3035" });

    expect(await tooltipFor(m.action_enter_coordinates())).toBe(
      m.action_enter_coordinates(),
    );
  });
});

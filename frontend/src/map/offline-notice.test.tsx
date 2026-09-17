import { beforeEach, describe, expect, it } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { OfflineNotice } from "./offline-notice";
import { useMapStore } from "./map-store";
import { m } from "@/i18n/messages";

/**
 * The notice a failed basemap gets instead of the "Map unavailable" panel.
 * jsdom computes no Tailwind, so what is asserted here is the semantics: that
 * it is a live status region rather than an alert, that it says the model is
 * unaffected, and that its one control puts the basemap back.
 */

beforeEach(() => {
  useMapStore.setState({ tilesFailed: false });
});

describe("OfflineNotice", () => {
  it("shows nothing while the tiles are arriving", () => {
    render(<OfflineNotice />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("announces a failed basemap as a status, not an alert", () => {
    // `alert` interrupts; this does not. The map is still usable, and the
    // model is still drawn — the notice exists so the blank background is not
    // read as lost data.
    useMapStore.setState({ tilesFailed: true });
    render(<OfflineNotice />);

    const notice = screen.getByRole("status", {
      name: m.label_basemap_offline(),
    });
    expect(notice).toHaveTextContent(m.msg_basemap_tiles_failed());
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("clears the flag on retry, which is what rebuilds the map", () => {
    useMapStore.setState({ tilesFailed: true });
    render(<OfflineNotice />);

    act(() => {
      fireEvent.click(screen.getByRole("button", { name: m.action_retry() }));
    });

    expect(useMapStore.getState().tilesFailed).toBe(false);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
  });

  it("offers the retry as a real button, reachable by keyboard", () => {
    // The map controls that refuse an action use `aria-disabled`; this one
    // refuses nothing, so it must simply be focusable and pressable.
    useMapStore.setState({ tilesFailed: true });
    render(<OfflineNotice />);

    const retry = screen.getByRole("button", { name: m.action_retry() });
    expect(retry).not.toHaveAttribute("aria-disabled");
    expect(retry).not.toBeDisabled();
    retry.focus();
    expect(retry).toHaveFocus();
  });
});

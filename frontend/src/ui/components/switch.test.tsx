import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Switch } from "./switch";

describe("Switch", () => {
  it("renders as a switch and toggles on click", async () => {
    const user = userEvent.setup();
    const onCheckedChange = vi.fn();
    render(<Switch aria-label="Dark mode" onCheckedChange={onCheckedChange} />);

    const control = screen.getByRole("switch", { name: "Dark mode" });
    expect(control).toHaveAttribute("aria-checked", "false");

    await user.click(control);
    expect(control).toHaveAttribute("aria-checked", "true");
    expect(onCheckedChange).toHaveBeenCalledWith(true);
  });

  it("reflects a controlled value", () => {
    render(<Switch aria-label="Dark mode" checked />);
    expect(screen.getByRole("switch", { name: "Dark mode" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
  });
});

import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { Checkbox } from "./checkbox";

describe("Checkbox", () => {
  it("toggles aria-checked on click and reports the change", async () => {
    const user = userEvent.setup();
    const onCheckedChange = vi.fn();
    render(<Checkbox aria-label="Enable" onCheckedChange={onCheckedChange} />);

    const box = screen.getByRole("checkbox", { name: "Enable" });
    expect(box).toHaveAttribute("aria-checked", "false");

    await user.click(box);
    expect(box).toHaveAttribute("aria-checked", "true");
    expect(onCheckedChange).toHaveBeenCalledWith(true);

    await user.click(box);
    expect(box).toHaveAttribute("aria-checked", "false");
    expect(onCheckedChange).toHaveBeenLastCalledWith(false);
  });

  it("toggles with the Space key", async () => {
    const user = userEvent.setup();
    render(<Checkbox aria-label="Enable" />);

    const box = screen.getByRole("checkbox", { name: "Enable" });
    box.focus();
    await user.keyboard(" ");
    expect(box).toHaveAttribute("aria-checked", "true");
  });

  it("does not toggle when disabled", async () => {
    const user = userEvent.setup();
    const onCheckedChange = vi.fn();
    render(
      <Checkbox
        aria-label="Enable"
        disabled
        onCheckedChange={onCheckedChange}
      />,
    );

    await user.click(screen.getByRole("checkbox", { name: "Enable" }));
    expect(onCheckedChange).not.toHaveBeenCalled();
  });
});

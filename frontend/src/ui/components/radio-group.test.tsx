import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RadioGroup, RadioGroupItem } from "./radio-group";

function renderGroup(onValueChange?: (value: string) => void) {
  return render(
    <RadioGroup
      defaultValue="a"
      aria-label="Choice"
      {...(onValueChange ? { onValueChange } : {})}
    >
      <RadioGroupItem value="a" aria-label="A" />
      <RadioGroupItem value="b" aria-label="B" />
      <RadioGroupItem value="c" aria-label="C" />
    </RadioGroup>,
  );
}

describe("RadioGroup", () => {
  it("exposes a radiogroup with the default item checked", () => {
    renderGroup();
    expect(screen.getByRole("radiogroup", { name: "Choice" })).toBeVisible();
    expect(screen.getByRole("radio", { name: "A" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(screen.getByRole("radio", { name: "B" })).toHaveAttribute(
      "aria-checked",
      "false",
    );
  });

  it("selects an item on click and reports the value", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    renderGroup(onValueChange);

    await user.click(screen.getByRole("radio", { name: "C" }));
    expect(screen.getByRole("radio", { name: "C" })).toHaveAttribute(
      "aria-checked",
      "true",
    );
    expect(onValueChange).toHaveBeenCalledWith("c");
  });

  it("moves the selection with ArrowDown", async () => {
    const user = userEvent.setup();
    const onValueChange = vi.fn();
    renderGroup(onValueChange);

    screen.getByRole("radio", { name: "A" }).focus();
    // Radix moves focus on the next tick and checks the newly focused item
    // only while an arrow key is still down, so the key is held across the
    // assertion — as a person's finger would be — and released afterwards.
    await user.keyboard("{ArrowDown>}");
    await waitFor(() => {
      expect(screen.getByRole("radio", { name: "B" })).toHaveAttribute(
        "aria-checked",
        "true",
      );
    });
    await user.keyboard("{/ArrowDown}");
    expect(onValueChange).toHaveBeenLastCalledWith("b");
  });
});

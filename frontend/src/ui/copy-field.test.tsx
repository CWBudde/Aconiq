import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { m } from "@/i18n/messages";
import { CopyButton, CopyField } from "./copy-field";

const toast = vi.hoisted(() => ({ error: vi.fn() }));

// The toast is the failure channel; sonner's own rendering is not under test.
vi.mock("sonner", () => ({ toast }));

const writeText = vi.fn<(text: string) => Promise<void>>();

beforeEach(() => {
  vi.useFakeTimers();
  writeText.mockReset();
  writeText.mockResolvedValue(undefined);
  toast.error.mockReset();
  // jsdom ships no clipboard; the component only needs `writeText`.
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });
});

afterEach(() => {
  vi.useRealTimers();
});

async function flushPromises() {
  await act(async () => {
    await Promise.resolve();
  });
}

describe("CopyButton", () => {
  it("writes the text and flashes the confirmation for 1.5 s", async () => {
    render(<CopyButton text="aconiq export --run-id r1" />);
    const button = screen.getByRole("button", { name: m.action_copy() });
    fireEvent.click(button);
    expect(writeText).toHaveBeenCalledWith("aconiq export --run-id r1");

    await flushPromises();
    expect(button).toHaveTextContent(m.status_copied());
    expect(button).toHaveAttribute("data-copied", "true");

    act(() => {
      vi.advanceTimersByTime(1499);
    });
    expect(button).toHaveTextContent(m.status_copied());
    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(button).toHaveTextContent(m.action_copy());
    expect(button).not.toHaveAttribute("data-copied");
  });

  it("uses the caller's label until copied", () => {
    render(<CopyButton text="x" label="Copy path" />);
    expect(screen.getByRole("button", { name: "Copy path" })).toBeVisible();
  });

  it("reports a refused clipboard write as a toast", async () => {
    writeText.mockRejectedValue(new Error("NotAllowedError"));
    render(<CopyButton text="secret" />);
    fireEvent.click(screen.getByRole("button", { name: m.action_copy() }));
    await flushPromises();
    expect(toast.error).toHaveBeenCalledWith(m.msg_copy_failed());
    expect(
      screen.getByRole("button", { name: m.action_copy() }),
    ).not.toHaveAttribute("data-copied");
  });

  it("keeps an accessible name when rendered icon-only", () => {
    render(<CopyButton text="x" size="icon" />);
    const button = screen.getByRole("button", { name: m.action_copy() });
    expect(button).toHaveTextContent("");
  });

  it("does not flip state after unmount", async () => {
    const { unmount } = render(<CopyButton text="x" />);
    fireEvent.click(screen.getByRole("button"));
    await flushPromises();
    unmount();
    // The timer is cleared on unmount: firing it must not touch state.
    expect(() => {
      vi.runOnlyPendingTimers();
    }).not.toThrow();
  });
});

describe("CopyField", () => {
  it("shows the value read-only with a copy button beside it", () => {
    render(
      <CopyField label="Command" value="aconiq run --standard rls19-road" />,
    );
    expect(screen.getByText("Command")).toBeVisible();
    const code = screen.getByText("aconiq run --standard rls19-road");
    expect(code.tagName).toBe("CODE");
    expect(code).toHaveClass("font-mono");
    fireEvent.click(screen.getByRole("button", { name: m.action_copy() }));
    expect(writeText).toHaveBeenCalledWith("aconiq run --standard rls19-road");
  });

  it("can render the value in the proportional face", () => {
    render(<CopyField value="plain" mono={false} />);
    expect(screen.getByText("plain")).not.toHaveClass("font-mono");
  });
});

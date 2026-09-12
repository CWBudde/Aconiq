import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { m } from "@/i18n/messages";
import { StatusBadge, type RunStatus } from "./status-badge";

const labels: Record<RunStatus, () => string> = {
  pending: m.status_badge_pending,
  running: m.status_badge_running,
  completed: m.status_badge_completed,
  failed: m.status_badge_failed,
};

describe("StatusBadge", () => {
  it.each(Object.keys(labels) as RunStatus[])(
    "names the %s state in words and exposes it as data",
    (status) => {
      render(<StatusBadge status={status} />);
      const badge = screen.getByText(labels[status]());
      expect(badge).toHaveAttribute("data-status", status);
      expect(badge).toHaveAttribute("data-slot", "badge");
    },
  );

  it("carries an icon for every state, hidden from the accessibility tree", () => {
    const { container } = render(<StatusBadge status="completed" />);
    const icon = container.querySelector("svg");
    expect(icon).not.toBeNull();
    expect(icon).toHaveAttribute("aria-hidden", "true");
  });

  it("spins the icon only while running", () => {
    const { container, rerender } = render(<StatusBadge status="running" />);
    expect(container.querySelector("svg")).toHaveClass("animate-spin");
    rerender(<StatusBadge status="pending" />);
    expect(container.querySelector("svg")).not.toHaveClass("animate-spin");
  });

  it("appends a caller class", () => {
    render(<StatusBadge status="failed" className="ml-2" />);
    expect(screen.getByText(m.status_badge_failed())).toHaveClass("ml-2");
  });
});

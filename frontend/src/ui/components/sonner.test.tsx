import { afterEach, describe, expect, it } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { toast } from "sonner";
import { ThemeProvider } from "@/ui/theme-provider";
import { Toaster } from "./sonner";

// sonner keeps its toasts in module state, so each test clears what it raised.
afterEach(() => {
  act(() => {
    toast.dismiss();
  });
});

describe("Toaster", () => {
  it("shows a toast raised through sonner's toast()", async () => {
    render(
      <ThemeProvider defaultTheme="light" storageKey="toaster-test">
        <Toaster />
      </ThemeProvider>,
    );

    act(() => {
      toast("Model saved");
    });

    expect(await screen.findByText("Model saved")).toBeInTheDocument();
  });

  it("takes its theme from the app's ThemeProvider", async () => {
    render(
      <ThemeProvider defaultTheme="dark" storageKey="toaster-test-dark">
        <Toaster />
      </ThemeProvider>,
    );

    // sonner only mounts its toast list once a toast exists, and stamps the
    // resolved theme on that list.
    act(() => {
      toast("Themed");
    });
    const item = await screen.findByText("Themed");
    expect(item.closest("ol")).toHaveAttribute("data-sonner-theme", "dark");
  });
});

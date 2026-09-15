import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { ItemList, ListItem, MasterDetail } from "./master-detail";

describe("MasterDetail", () => {
  it("renders the header, the list and the detail pane", () => {
    render(
      <MasterDetail
        listLabel="Runs"
        header={<h2>Runs</h2>}
        list={<p>list body</p>}
      >
        <p>detail body</p>
      </MasterDetail>,
    );
    const column = screen.getByRole("complementary", { name: "Runs" });
    expect(column).toContainElement(
      screen.getByRole("heading", { name: "Runs" }),
    );
    expect(column).toContainElement(screen.getByText("list body"));
    expect(column).not.toContainElement(screen.getByText("detail body"));
  });

  it("takes the list width from the caller", () => {
    render(
      <MasterDetail listWidth="w-80" list={null}>
        x
      </MasterDetail>,
    );
    expect(screen.getByRole("complementary")).toHaveClass("w-80");
    expect(screen.getByRole("complementary")).not.toHaveClass("w-72");
  });
});

describe("ItemList and ListItem", () => {
  function renderList(
    selectedId: string,
    onSelect = vi.fn<(id: string) => void>(),
  ) {
    return render(
      <ItemList>
        {["a", "b"].map((id) => (
          <ListItem
            key={id}
            selected={id === selectedId}
            onSelect={() => {
              onSelect(id);
            }}
            title={`Run ${id}`}
            code={id}
            meta="12 sec"
          />
        ))}
      </ItemList>,
    );
  }

  it("exposes the rows as a list of list items holding buttons", () => {
    renderList("a");
    expect(screen.getByRole("list")).toBeVisible();
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    expect(screen.getAllByRole("button")).toHaveLength(2);
  });

  it("marks the selected row with aria-current only", () => {
    renderList("b");
    const [first, second] = screen.getAllByRole("button");
    expect(first).not.toHaveAttribute("aria-current");
    expect(second).toHaveAttribute("aria-current", "true");
    expect(second).toHaveClass("bg-accent");
    expect(first).not.toHaveClass("bg-accent");
  });

  it("selects on click", () => {
    const onSelect = vi.fn();
    renderList("a", onSelect);
    fireEvent.click(screen.getByRole("button", { name: /Run b/ }));
    expect(onSelect).toHaveBeenCalledWith("b");
  });

  it("renders the badge, code, title and meta slots", () => {
    render(
      <ItemList>
        <ListItem
          selected={false}
          onSelect={() => undefined}
          badge={<span data-testid="badge">Done</span>}
          code="run-1"
          title="rls19-road / 2019"
          meta="13:05:07 · 2 min 5 sec"
        />
      </ItemList>,
    );
    const button = screen.getByRole("button");
    expect(button).toContainElement(screen.getByTestId("badge"));
    expect(screen.getByText("run-1")).toHaveClass("font-mono");
    expect(button).toHaveTextContent("rls19-road / 2019");
    expect(button).toHaveTextContent("13:05:07 · 2 min 5 sec");
  });

  it("can drop the chevron", () => {
    const { container } = render(
      <ItemList>
        <ListItem
          selected={false}
          onSelect={() => undefined}
          title="No chevron"
          chevron={false}
        />
      </ItemList>,
    );
    expect(container.querySelector("svg")).toBeNull();
  });
});

describe("ListItem as a link", () => {
  function renderRows() {
    return render(
      <MemoryRouter>
        <ItemList label="Runs">
          <ListItem selected to="/results/a" title="Run A" />
          <ListItem selected={false} to="/results/b" title="Run B" />
        </ItemList>
      </MemoryRouter>,
    );
  }

  it("renders a row that targets a route as a real link", () => {
    renderRows();
    const rows = screen.getAllByRole("link");
    expect(rows).toHaveLength(2);
    expect(rows[0]).toHaveAttribute("href", "/results/a");
  });

  it("marks the selected link as the current page, not merely as true", () => {
    // A selected link IS the current page. `aria-current="true"` announces
    // "current" where "page" announces "current page", and axe accepts both —
    // so nothing but this assertion catches the drift.
    renderRows();
    const [first, second] = screen.getAllByRole("link");
    expect(first).toHaveAttribute("aria-current", "page");
    expect(second).not.toHaveAttribute("aria-current");
  });

  it("keeps the selected styling hook on the link variant", () => {
    renderRows();
    const [first] = screen.getAllByRole("link");
    expect(first).toHaveAttribute("data-selected", "true");
    expect(first?.className).toContain("bg-accent");
  });

  it("still renders a button when the row only changes local state", () => {
    render(
      <ItemList label="Runs">
        <ListItem selected onSelect={() => undefined} title="Run A" />
      </ItemList>,
    );
    expect(screen.getByRole("button")).toHaveAttribute("aria-current", "true");
    expect(screen.queryByRole("link")).toBeNull();
  });
});

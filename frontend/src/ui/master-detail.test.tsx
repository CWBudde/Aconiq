import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
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

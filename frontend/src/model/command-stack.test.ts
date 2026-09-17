import { describe, expect, it, vi } from "vitest";
import { CommandStack, type Command } from "./command-stack";

function makeCommand(log: string[]): Command {
  return {
    description: "test",
    execute: vi.fn(() => {
      log.push("do");
    }),
    undo: vi.fn(() => {
      log.push("undo");
    }),
  };
}

/**
 * One step of a drag, against a single-valued model.
 *
 * `previous` is read at construction time and not at execute time, which is
 * what `model-store`'s geometry commands do and what the merge rule exists for:
 * every step of a drag holds the value it personally replaced, so only the
 * first one can restore the value the drag started from.
 */
function makeSetter(
  cell: { value: string },
  next: string,
  coalesceKey?: string,
): Command {
  const previous = cell.value;
  return {
    description: `set ${next}`,
    ...(coalesceKey === undefined ? {} : { coalesceKey }),
    execute: vi.fn(() => {
      cell.value = next;
    }),
    undo: vi.fn(() => {
      cell.value = previous;
    }),
  };
}

function countUndoSteps(stack: CommandStack): number {
  let steps = 0;
  while (stack.canUndo()) {
    stack.undo();
    steps++;
  }
  return steps;
}

describe("CommandStack", () => {
  it("starts with empty undo/redo", () => {
    const stack = new CommandStack();
    expect(stack.canUndo()).toBe(false);
    expect(stack.canRedo()).toBe(false);
  });

  it("execute runs the command and enables undo", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    const cmd = makeCommand(log);
    stack.execute(cmd);
    expect(log).toEqual(["do"]);
    expect(stack.canUndo()).toBe(true);
    expect(stack.canRedo()).toBe(false);
  });

  it("undo reverses the last command", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    stack.execute(makeCommand(log));
    stack.undo();
    expect(log).toEqual(["do", "undo"]);
    expect(stack.canUndo()).toBe(false);
    expect(stack.canRedo()).toBe(true);
  });

  it("redo re-applies the last undone command", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    stack.execute(makeCommand(log));
    stack.undo();
    stack.redo();
    expect(log).toEqual(["do", "undo", "do"]);
    expect(stack.canUndo()).toBe(true);
    expect(stack.canRedo()).toBe(false);
  });

  it("new command after undo clears redo stack", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    stack.execute(makeCommand(log));
    stack.undo();
    stack.execute(makeCommand(log));
    expect(stack.canRedo()).toBe(false);
  });

  it("respects max stack size", () => {
    const log: string[] = [];
    const stack = new CommandStack(3);
    for (let i = 0; i < 5; i++) {
      stack.execute(makeCommand(log));
    }
    let undoCount = 0;
    while (stack.canUndo()) {
      stack.undo();
      undoCount++;
    }
    expect(undoCount).toBe(3);
  });

  it("clear removes all history", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    stack.execute(makeCommand(log));
    stack.clear();
    expect(stack.canUndo()).toBe(false);
    expect(stack.canRedo()).toBe(false);
  });

  it("notifies listener on changes", () => {
    const stack = new CommandStack();
    const listener = vi.fn();
    stack.subscribe(listener);
    stack.execute(makeCommand([]));
    expect(listener).toHaveBeenCalledTimes(1);
    stack.undo();
    expect(listener).toHaveBeenCalledTimes(2);
  });

  it("unsubscribe stops notifications", () => {
    const stack = new CommandStack();
    const listener = vi.fn();
    const unsub = stack.subscribe(listener);
    unsub();
    stack.execute(makeCommand([]));
    expect(listener).not.toHaveBeenCalled();
  });
});

describe("CommandStack coalescing", () => {
  it("merges consecutive commands carrying the same key into one step", () => {
    // What a drag emits: one command per mousemove. Without the key each of
    // them is an undo step, so Ctrl+Z walks the shape back a pixel at a time.
    const cell = { value: "a" };
    const stack = new CommandStack();
    stack.execute(makeSetter(cell, "b", "geometry:f1"));
    stack.execute(makeSetter(cell, "c", "geometry:f1"));
    stack.execute(makeSetter(cell, "d", "geometry:f1"));

    expect(cell.value).toBe("d");
    expect(countUndoSteps(stack)).toBe(1);
  });

  it("undoes a merged chain to the value it started from", () => {
    // The merge keeps the *oldest* `undo`. Each step captured the value it
    // personally replaced, so keeping the newest would undo the drag to one
    // mousemove before its end — a shape almost where the user left it, which
    // reads as an undo that did nothing.
    const cell = { value: "a" };
    const stack = new CommandStack();
    stack.execute(makeSetter(cell, "b", "geometry:f1"));
    stack.execute(makeSetter(cell, "c", "geometry:f1"));
    stack.execute(makeSetter(cell, "d", "geometry:f1"));

    stack.undo();

    expect(cell.value).toBe("a");
  });

  it("redoes a merged chain to the value it ended at", () => {
    // And the *newest* `execute`, for the mirrored reason.
    const cell = { value: "a" };
    const stack = new CommandStack();
    stack.execute(makeSetter(cell, "b", "geometry:f1"));
    stack.execute(makeSetter(cell, "c", "geometry:f1"));
    stack.execute(makeSetter(cell, "d", "geometry:f1"));

    stack.undo();
    stack.redo();

    expect(cell.value).toBe("d");
    expect(stack.canRedo()).toBe(false);
    expect(stack.canUndo()).toBe(true);
  });

  it("merging still clears the redo stack", () => {
    const cell = { value: "a" };
    const stack = new CommandStack();
    stack.execute(makeSetter(cell, "b", "geometry:f1"));
    stack.execute(makeSetter(cell, "c", "geometry:f1"));
    stack.undo();
    expect(stack.canRedo()).toBe(true);

    stack.execute(makeSetter(cell, "d", "geometry:f1"));

    expect(stack.canRedo()).toBe(false);
  });

  it("seal breaks the chain without touching the stacks", () => {
    // The gesture ended. The stack cannot see that — a second drag of the same
    // feature carries the same key and looks like a continuation — so whoever
    // knows says so.
    const cell = { value: "a" };
    const stack = new CommandStack();
    stack.execute(makeSetter(cell, "b", "geometry:f1"));
    stack.execute(makeSetter(cell, "c", "geometry:f1"));

    stack.seal();
    expect(stack.canUndo()).toBe(true);
    expect(stack.canRedo()).toBe(false);

    stack.execute(makeSetter(cell, "d", "geometry:f1"));

    stack.undo();
    expect(cell.value).toBe("c");
    stack.undo();
    expect(cell.value).toBe("a");
    expect(stack.canUndo()).toBe(false);
  });

  it("never merges across different keys", () => {
    // Two features dragged in succession. Merging them would restore the first
    // feature's starting shape onto the second.
    const cell = { value: "a" };
    const stack = new CommandStack();
    stack.execute(makeSetter(cell, "b", "geometry:f1"));
    stack.execute(makeSetter(cell, "c", "geometry:f2"));
    stack.execute(makeSetter(cell, "d", "geometry:f1"));

    expect(countUndoSteps(stack)).toBe(3);
    expect(cell.value).toBe("a");
  });

  it("never merges a command that carries no key", () => {
    const log: string[] = [];
    const stack = new CommandStack();
    stack.execute(makeCommand(log));
    stack.execute(makeCommand(log));

    expect(countUndoSteps(stack)).toBe(2);
  });

  it("a key-less command in between breaks the chain", () => {
    const cell = { value: "a" };
    const stack = new CommandStack();
    stack.execute(makeSetter(cell, "b", "geometry:f1"));
    stack.execute(makeSetter(cell, "c"));
    stack.execute(makeSetter(cell, "d", "geometry:f1"));

    expect(countUndoSteps(stack)).toBe(3);
  });

  it("a merged chain costs one slot against the max size", () => {
    // Replacing the top rather than pushing onto it is what keeps a long drag
    // from evicting the rest of the session's history.
    const cell = { value: "a" };
    const stack = new CommandStack(3);
    stack.execute(makeSetter(cell, "b"));
    for (let i = 0; i < 20; i++) {
      stack.execute(makeSetter(cell, `drag-${String(i)}`, "geometry:f1"));
    }

    expect(countUndoSteps(stack)).toBe(2);
    expect(cell.value).toBe("a");
  });

  it("an undo closes the chain, so the next command is its own step", () => {
    // The command the chain was open against has left the undo stack; merging
    // into whatever is underneath would attach this edit's redo to another
    // edit's undo.
    const cell = { value: "a" };
    const stack = new CommandStack();
    stack.execute(makeSetter(cell, "b", "geometry:f1"));
    stack.execute(makeSetter(cell, "c", "geometry:f1"));
    stack.undo();
    expect(cell.value).toBe("a");

    stack.execute(makeSetter(cell, "d", "geometry:f1"));

    expect(countUndoSteps(stack)).toBe(1);
    expect(cell.value).toBe("a");
  });
});

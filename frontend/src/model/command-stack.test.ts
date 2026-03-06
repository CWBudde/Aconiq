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

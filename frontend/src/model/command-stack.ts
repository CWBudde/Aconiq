export interface Command {
  description: string;
  execute: () => void;
  undo: () => void;
  /**
   * Makes this command a continuation of the one before it rather than a step
   * of its own.
   *
   * Two commands carrying the same non-empty key, executed back to back, are
   * merged into one undo step — see {@link CommandStack.execute} for what the
   * merged command keeps. It exists for input that arrives as a stream of
   * intermediate states: a feature dragged across the map emits one geometry
   * per mousemove, and without a key each of those is an undo step, so Ctrl+Z
   * walks the shape back a pixel at a time instead of undoing the drag.
   *
   * The key has to name the thing being edited rather than the kind of edit —
   * `geometry:<feature id>` and not `geometry` — or two features dragged in
   * succession would collapse into one step, and undoing it would restore the
   * first feature's starting shape onto the second.
   *
   * A chain is broken by a differing key, by {@link CommandStack.seal}, and by
   * anything that moves the stacks under it (undo, redo, clear).
   */
  coalesceKey?: string;
}

type Listener = () => void;

export class CommandStack {
  private undoStack: Command[] = [];
  private redoStack: Command[] = [];
  private readonly maxSize: number;
  private listeners: Listener[] = [];
  /**
   * The key the top of the undo stack is still open for merging under, or
   * `null` when nothing is. Kept apart from the top command's own key so that
   * an undo, a redo or a {@link seal} can close the chain without rewriting
   * what is on the stack.
   */
  private openKey: string | null = null;

  constructor(maxSize = 50) {
    this.maxSize = maxSize;
  }

  /**
   * Runs the command and pushes it — or, where it continues an open chain,
   * merges it into the step already there.
   *
   * A merge keeps the **newest `execute`** and the **oldest `undo`**, and that
   * asymmetry is the whole point. `updateFeature` and its geometry siblings
   * capture `previous` *before* the command runs, so every step of a drag holds
   * the state it personally replaced: only the first one holds the geometry the
   * drag started from, and only the last one holds the geometry it ended at.
   * Keeping the last `undo` would undo a drag back to one mousemove before its
   * end; keeping the first `execute` would redo it to one mousemove after its
   * start.
   *
   * The merged step replaces the top rather than being pushed onto it, so a
   * drag of any length costs one slot and cannot push older history out past
   * `maxSize`. It still clears the redo stack: a merged command is an edit like
   * any other, and a redo surviving one would re-apply a future the model has
   * left.
   */
  execute(command: Command): void {
    command.execute();

    const key = command.coalesceKey ?? "";
    const top = this.undoStack.at(-1);

    if (key !== "" && key === this.openKey && top !== undefined) {
      this.undoStack[this.undoStack.length - 1] = {
        description: command.description,
        execute: command.execute,
        undo: top.undo,
        coalesceKey: key,
      };
    } else {
      this.undoStack.push(command);
      if (this.undoStack.length > this.maxSize) {
        this.undoStack.shift();
      }
    }

    this.openKey = key === "" ? null : key;
    this.redoStack = [];
    this.notify();
  }

  /**
   * Closes the open coalescing chain without touching either stack.
   *
   * What follows starts a new undo step even when it carries the same key. The
   * caller is whoever knows that an edit gesture has ended — the feature was
   * deselected, the tool was put away, the panel was closed — which the stack
   * itself cannot see: to it, a second drag of the same feature looks exactly
   * like a continuation of the first.
   */
  seal(): void {
    this.openKey = null;
  }

  undo(): void {
    const command = this.undoStack.pop();
    if (!command) return;
    // The chain ends here whatever it was. The command it was open against has
    // left the undo stack, so a later command carrying the same key would
    // merge into whatever now happens to be underneath — a different edit,
    // holding a different `undo`.
    this.openKey = null;
    command.undo();
    this.redoStack.push(command);
    this.notify();
  }

  redo(): void {
    const command = this.redoStack.pop();
    if (!command) return;
    this.openKey = null;
    command.execute();
    this.undoStack.push(command);
    this.notify();
  }

  canUndo(): boolean {
    return this.undoStack.length > 0;
  }

  canRedo(): boolean {
    return this.redoStack.length > 0;
  }

  clear(): void {
    this.undoStack = [];
    this.redoStack = [];
    this.openKey = null;
    this.notify();
  }

  subscribe(listener: Listener): () => void {
    this.listeners.push(listener);
    return () => {
      this.listeners = this.listeners.filter((l) => l !== listener);
    };
  }

  private notify(): void {
    for (const listener of this.listeners) {
      listener();
    }
  }
}

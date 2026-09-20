import { describe, expect, it } from "vitest";
import {
  cursorFor,
  cursorPosition,
  findingQueue,
  stepFinding,
} from "./finding-queue";
import type { ValidationIssue, ValidationReport } from "./types";

function warning(featureId: string): ValidationIssue {
  return {
    level: "warning",
    code: "source.rls19.review_required",
    featureId,
    params: {},
  };
}

function error(featureId: string): ValidationIssue {
  return {
    level: "error",
    code: "building.height.required",
    featureId,
    params: {},
  };
}

function report(
  errors: ValidationIssue[],
  warnings: ValidationIssue[],
): ValidationReport {
  return {
    valid: errors.length === 0 && warnings.length === 0,
    errors,
    warnings,
    checkedAt: "2026-09-20T00:00:00.000Z",
  };
}

describe("findingQueue", () => {
  it("is empty when nothing was validated", () => {
    expect(findingQueue(null)).toEqual([]);
  });

  it("lists the errors before the warnings", () => {
    const queue = findingQueue(
      report([error("bld-1")], [warning("road-1"), warning("road-2")]),
    );
    expect(queue).toEqual(["bld-1", "road-1", "road-2"]);
  });

  it("is one stop per feature, not one per finding", () => {
    // A road missing both its speed and its surface carries two findings. The
    // editor shows both at once, so "next" must not sit still on it.
    const queue = findingQueue(
      report([], [warning("road-1"), warning("road-1"), warning("road-2")]),
    );
    expect(queue).toEqual(["road-1", "road-2"]);
  });

  it("drops a finding that names no feature to travel to", () => {
    const modelEmpty: ValidationIssue = {
      level: "error",
      code: "model.empty",
      featureId: "",
      params: {},
    };
    expect(findingQueue(report([modelEmpty], []))).toEqual([]);
  });
});

describe("cursorFor", () => {
  it("finds the feature's place in the queue", () => {
    expect(cursorFor(["a", "b", "c"], "b")).toEqual({
      featureId: "b",
      index: 1,
    });
  });

  it("answers null for a feature the queue does not hold", () => {
    expect(cursorFor(["a", "b"], "z")).toBeNull();
  });
});

describe("stepFinding", () => {
  const queue = ["a", "b", "c"];

  it("has nowhere to go in an empty queue", () => {
    expect(stepFinding([], { featureId: "a", index: 0 }, 1)).toBeNull();
  });

  it("starts at either end when nothing is selected yet", () => {
    expect(stepFinding(queue, null, 1)).toEqual({ featureId: "a", index: 0 });
    expect(stepFinding(queue, null, -1)).toEqual({ featureId: "c", index: 2 });
  });

  it("walks forwards and backwards", () => {
    expect(stepFinding(queue, { featureId: "a", index: 0 }, 1)).toEqual({
      featureId: "b",
      index: 1,
    });
    expect(stepFinding(queue, { featureId: "b", index: 1 }, -1)).toEqual({
      featureId: "a",
      index: 0,
    });
  });

  it("wraps at both ends", () => {
    expect(stepFinding(queue, { featureId: "c", index: 2 }, 1)).toEqual({
      featureId: "a",
      index: 0,
    });
    expect(stepFinding(queue, { featureId: "a", index: 0 }, -1)).toEqual({
      featureId: "c",
      index: 2,
    });
  });

  it("lands on the next unfixed finding when the current one was fixed", () => {
    // The reader stood on "b" (index 1) and corrected it, so "b" left the
    // queue and "c" moved into its slot. Forward must be "c" — recomputing
    // from the id would find nothing, and skipping to index 2 would miss it.
    expect(stepFinding(["a", "c"], { featureId: "b", index: 1 }, 1)).toEqual({
      featureId: "c",
      index: 1,
    });
  });

  it("goes back to the one before the fixed finding's slot", () => {
    expect(stepFinding(["a", "c"], { featureId: "b", index: 1 }, -1)).toEqual({
      featureId: "a",
      index: 0,
    });
  });

  it("clamps when the fixed finding was the last one", () => {
    expect(stepFinding(["a"], { featureId: "c", index: 2 }, 1)).toEqual({
      featureId: "a",
      index: 0,
    });
  });
});

describe("cursorPosition", () => {
  it("has no position without a cursor or without a queue", () => {
    expect(cursorPosition(["a"], null)).toBeNull();
    expect(cursorPosition([], { featureId: "a", index: 0 })).toBeNull();
  });

  it("is 1-based for a finding that is still open", () => {
    expect(cursorPosition(["a", "b", "c"], { featureId: "b", index: 1 })).toBe(
      2,
    );
  });

  it("follows the finding when an earlier one is fixed", () => {
    // "a" was corrected, so "b" is now first. The stored index still says 1.
    expect(cursorPosition(["b", "c"], { featureId: "b", index: 1 })).toBe(1);
  });

  it("never reads past the end of the queue", () => {
    // The reader corrected the last of 608 findings. Rendering the stored
    // index raw is how this used to say "608 / 607".
    expect(cursorPosition(["a"], { featureId: "b", index: 1 })).toBe(1);
  });

  it("points at the finding that took the fixed one's place", () => {
    // Which is also where `stepFinding` goes next, so the number the reader
    // sees and the one "next" acts on cannot disagree.
    const queue = ["a", "c"];
    const cursor = { featureId: "b", index: 1 };
    expect(cursorPosition(queue, cursor)).toBe(2);
    expect(stepFinding(queue, cursor, 1)).toEqual({
      featureId: "c",
      index: 1,
    });
  });
});

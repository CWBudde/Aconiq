import type { ValidationReport } from "./types";

/**
 * Where the reader stands in the queue.
 *
 * It carries the index as well as the id because the queue changes underneath
 * it: the reader's whole purpose is to *fix* the finding they are standing on,
 * and a fixed finding leaves the queue. See {@link stepFinding}.
 */
export interface FindingCursor {
  featureId: string;
  index: number;
}

/**
 * The features a reader has to visit, in the order the report lists them.
 *
 * Distinct feature ids, not one entry per finding. A feature carrying three
 * findings is one stop on the walk — the docked editor shows all three at once
 * when the reader lands on it — and one entry per finding would make "next"
 * appear to do nothing twice in a row, which reads as a broken button.
 *
 * Findings with no `featureId` are dropped, which is the rule the validation
 * panel's "go to" already applies: `model.empty` names nothing to travel to.
 */
export function findingQueue(report: ValidationReport | null): string[] {
  if (report === null) return [];

  const seen = new Set<string>();
  const queue: string[] = [];
  for (const issue of [...report.errors, ...report.warnings]) {
    if (issue.featureId === "") continue;
    if (seen.has(issue.featureId)) continue;
    seen.add(issue.featureId);
    queue.push(issue.featureId);
  }
  return queue;
}

/** Where the cursor lands after a step, or `null` once the queue is empty. */
export function stepFinding(
  queue: string[],
  cursor: FindingCursor | null,
  direction: 1 | -1,
): FindingCursor | null {
  if (queue.length === 0) return null;

  const at = cursor ? queue.indexOf(cursor.featureId) : -1;

  let index: number;
  if (at >= 0) {
    // The ordinary case: the finding the reader is on is still open. Wrapping
    // is deliberate — at 608 findings someone reaches the end, and the
    // "12 / 608" readout is what makes the wrap legible rather than confusing.
    index = (at + direction + queue.length) % queue.length;
  } else if (cursor === null) {
    index = direction === 1 ? 0 : queue.length - 1;
  } else {
    // The finding was fixed and has left the queue, which is the *common* case
    // here: the reader corrected the acoustics and pressed "next". Everything
    // after it shifted down one, so the slot the cursor held now contains what
    // used to follow — stepping forward means staying at that index, and
    // stepping back means the one before it. Landing on the next unfixed
    // finding is the whole point; recomputing from the id would find nothing.
    index =
      direction === 1
        ? Math.min(cursor.index, queue.length - 1)
        : Math.max(cursor.index - 1, 0);
  }

  const featureId = queue[index];
  if (featureId === undefined) return null;
  return { featureId, index };
}

/** The cursor that starts a walk at `featureId`, or `null` if it is not queued. */
export function cursorFor(
  queue: string[],
  featureId: string,
): FindingCursor | null {
  const index = queue.indexOf(featureId);
  if (index < 0) return null;
  return { featureId, index };
}

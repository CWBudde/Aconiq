// What a kernel refusal is, on both sides of the worker boundary.
//
// Two problems meet here, and one function solves both.
//
// The first is that `backend/cmd/wasm/main.go` rejects with a **bare string**,
// not an Error: `reject.Invoke(js.ValueOf(fmt.Sprintf(...)))`. So `await
// kernel.rls19Road(...)` has always thrown a string, and every consumer that
// reads `error.message` off it — `run/setup-dialog.tsx` renders exactly that —
// got `undefined` and drew an empty red callout. A browser-mode refusal that
// names the defect in the same words `aconiq run` prints was arriving intact
// and rendering as nothing.
//
// The second is that an `Error` does not survive `postMessage`'s structured
// clone with its identity intact across every engine, and several of those
// sentences are deliberately verbatim copies of the CLI's — `transformFunc`
// and `contoursFunc` say so in their own comments, and
// `api/browser-backend.ts` relies on the wording. So the worker sends the
// string and the client rebuilds the Error, rather than trusting the clone.
//
// Both kernels use these: `kernel-node.ts` normalises the same way, so a test
// asserting on a rejection is asserting on what the browser actually throws.

/**
 * The message to put on the wire for whatever a kernel export rejected with.
 *
 * Go's bare strings pass through unchanged — they are already the sentence —
 * and an Error contributes its `message` for the same reason. Anything else is
 * stringified rather than dropped: an unrecognisable rejection reaching the
 * user as its own text is worse than useless only if it arrives empty.
 */
export function serializeKernelError(cause: unknown): string {
  if (typeof cause === "string") return cause;
  if (cause instanceof Error) return cause.message;
  if (cause === undefined || cause === null) {
    return "the kernel rejected without saying why";
  }
  if (typeof cause === "object") {
    // Not `String(cause)`: an object with no `message` stringifies to
    // "[object Object]", which is the empty callout wearing a different hat.
    try {
      return `the kernel rejected with ${JSON.stringify(cause)}`;
    } catch {
      return "the kernel rejected with an object that cannot be described";
    }
  }
  if (
    typeof cause === "number" ||
    typeof cause === "bigint" ||
    typeof cause === "boolean"
  ) {
    return String(cause);
  }
  return `the kernel rejected with a ${typeof cause}`;
}

/**
 * Rebuild the refusal as an `Error` carrying Go's sentence verbatim.
 *
 * Verbatim matters: `geo.ComputeCRSForGeographic` names the zone and says what
 * to do about it, and `contour.FromRaster`'s refusals are the ones `aconiq
 * export` prints. A reader comparing browser mode against a CLI bundle has to
 * read the same sentence in both, so nothing is prefixed or reworded here.
 */
export function reviveKernelError(message: string): Error {
  return new Error(message);
}

/**
 * Run a kernel export and make sure what it throws is an `Error`.
 *
 * This is the whole fix for the bare-string rejection, applied at the one
 * place both kernels call into Go.
 */
export async function withKernelErrors<T>(work: Promise<T>): Promise<T> {
  try {
    return await work;
  } catch (cause) {
    throw reviveKernelError(serializeKernelError(cause));
  }
}

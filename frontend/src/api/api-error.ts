/**
 * The local API answers a refused request with an error envelope — `code`,
 * `message`, `details` and `hint` — and the code is the only part of it a UI
 * may branch on. Collapsing that envelope into `new Error(message)` (as every
 * caller used to) throws away exactly the fields that let a page explain a
 * refusal instead of reporting a generic failure.
 */

import type { APIError } from "./client";

/**
 * A run against a scaffold-tier standard was refused because the request did
 * not acknowledge the tier. Mirrors `errorCodeExperimentalOptInRequired` in
 * `backend/internal/api/httpv1/handler.go`.
 */
export const ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED =
  "experimental_opt_in_required";

/** An error envelope from the local API, kept whole. */
export class APIRequestError extends Error {
  readonly code: string;
  readonly hint: string | undefined;
  readonly details: Record<string, unknown> | undefined;

  constructor(error: APIError) {
    super(error.message);
    this.name = "APIRequestError";
    this.code = error.code;
    this.hint = error.hint;
    this.details = error.details;
  }
}

/**
 * Narrow an unknown error — react-query types a mutation error as `Error` —
 * to an API envelope. Returns `null` for anything else: a network failure or
 * a thrown string carries no code, and must not be treated as if it did.
 */
export function asAPIRequestError(error: unknown): APIRequestError | null {
  return error instanceof APIRequestError ? error : null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

/**
 * Read an error envelope out of a parsed response body.
 *
 * Returns `null` unless both required fields are present and are strings: a
 * body that is not an envelope must not be dressed up as one, because a
 * fabricated `code` would be branched on downstream.
 */
export function parseErrorEnvelope(payload: unknown): APIError | null {
  if (!isRecord(payload)) return null;

  const error: unknown = payload.error;
  if (!isRecord(error)) return null;

  const { code, message, hint, details } = error;
  if (typeof code !== "string" || typeof message !== "string") return null;

  return {
    code,
    message,
    ...(typeof hint === "string" ? { hint } : {}),
    ...(isRecord(details) ? { details } : {}),
  };
}

/**
 * Build the error to throw for a non-OK response: the envelope when the body
 * carries one, and a plain status error otherwise.
 */
export async function errorFromResponse(response: Response): Promise<Error> {
  const payload: unknown = await response.json().catch(() => null);
  const envelope = parseErrorEnvelope(payload);
  if (envelope !== null) return new APIRequestError(envelope);
  return new Error(`Request failed: ${String(response.status)}`);
}

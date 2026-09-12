import { describe, expect, it } from "vitest";
import {
  APIRequestError,
  asAPIRequestError,
  ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
  ERROR_CODE_MODEL_INVALID,
  errorFromResponse,
  modelValidationIssues,
  parseErrorEnvelope,
} from "./api-error";

describe("parseErrorEnvelope", () => {
  it("keeps code, message, hint and details", () => {
    const envelope = parseErrorEnvelope({
      error: {
        code: ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
        message: "standard is scaffold tier",
        hint: 'Set "experimental": true.',
        details: { standard_id: "cnossos-road", evidence_tier: "scaffold" },
      },
    });

    expect(envelope).toEqual({
      code: ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
      message: "standard is scaffold tier",
      hint: 'Set "experimental": true.',
      details: { standard_id: "cnossos-road", evidence_tier: "scaffold" },
    });
  });

  it("omits an absent hint rather than carrying undefined", () => {
    const envelope = parseErrorEnvelope({
      error: { code: "bad_request", message: "no" },
    });

    expect(envelope).not.toBeNull();
    expect(envelope).not.toHaveProperty("hint");
  });

  it.each([
    ["a non-object body", "boom"],
    ["a body with no error member", { ok: true }],
    ["an envelope without a code", { error: { message: "no" } }],
    ["an envelope without a message", { error: { code: "bad_request" } }],
    ["a non-string code", { error: { code: 400, message: "no" } }],
  ])("refuses to invent an envelope from %s", (_label, payload) => {
    expect(parseErrorEnvelope(payload)).toBeNull();
  });
});

describe("errorFromResponse", () => {
  it("returns the envelope when the body carries one", async () => {
    const response = new Response(
      JSON.stringify({
        error: {
          code: ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED,
          message: "standard is scaffold tier",
          hint: 'Set "experimental": true.',
        },
      }),
      { status: 400 },
    );

    const error = await errorFromResponse(response);
    const apiError = asAPIRequestError(error);

    expect(apiError).toBeInstanceOf(APIRequestError);
    expect(apiError?.code).toBe(ERROR_CODE_EXPERIMENTAL_OPT_IN_REQUIRED);
    expect(apiError?.hint).toBe('Set "experimental": true.');
    expect(error.message).toBe("standard is scaffold tier");
  });

  it("falls back to the status for a body that is not an envelope", async () => {
    const response = new Response("<html>gateway</html>", { status: 502 });

    const error = await errorFromResponse(response);

    expect(asAPIRequestError(error)).toBeNull();
    expect(error.message).toBe("Request failed: 502");
  });
});

describe("asAPIRequestError", () => {
  it("does not treat a plain error as an envelope", () => {
    expect(asAPIRequestError(new Error("network down"))).toBeNull();
    expect(asAPIRequestError("network down")).toBeNull();
    expect(asAPIRequestError(null)).toBeNull();
  });
});

describe("modelValidationIssues", () => {
  function refusal(details: Record<string, unknown> | undefined) {
    return new APIRequestError({
      code: ERROR_CODE_MODEL_INVALID,
      message: "model validation failed",
      ...(details === undefined ? {} : { details }),
    });
  }

  it("lists the findings, keeping feature_id only where present", () => {
    expect(
      modelValidationIssues(
        refusal({
          errors: [
            { code: "a", message: "first", feature_id: "f1" },
            { code: "b", message: "second" },
          ],
        }),
      ),
    ).toEqual([
      { code: "a", message: "first", feature_id: "f1" },
      { code: "b", message: "second" },
    ]);
  });

  it("returns null for an error that is not a model refusal", () => {
    expect(modelValidationIssues(new Error("Request failed: 500"))).toBeNull();
    expect(
      modelValidationIssues(
        new APIRequestError({ code: "not_found", message: "no project" }),
      ),
    ).toBeNull();
  });

  it("returns null when the refusal names no findings", () => {
    // A "Show details" button over an empty list would be a dead end, so
    // the empty cases collapse into "nothing to show".
    expect(modelValidationIssues(refusal(undefined))).toBeNull();
    expect(modelValidationIssues(refusal({}))).toBeNull();
    expect(modelValidationIssues(refusal({ errors: [] }))).toBeNull();
    expect(modelValidationIssues(refusal({ errors: "oops" }))).toBeNull();
  });

  it("drops entries that lack a code or a message", () => {
    expect(
      modelValidationIssues(
        refusal({
          errors: [{ code: "a" }, { message: "m" }, 42, null],
        }),
      ),
    ).toBeNull();
  });
});

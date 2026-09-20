/**
 * The hint is the half that tells the reader what to do next.
 *
 * `POST /api/v1/import/osm` distinguishes a rate limit, a refusal and an
 * outage — all three arrive as `upstream_error` with the upstream status in
 * `details` and the remedy in `hint`. Rendering `err.message` alone put the
 * reader back where that change was meant to move them from: "failed with
 * status 429" says nothing about whether waiting will help.
 */

import { describe, expect, it } from "vitest";
import { APIRequestError } from "@/api/api-error";
import { osmFailureText } from "./osm-import";

describe("osmFailureText", () => {
  it("shows the hint beside the message", () => {
    const text = osmFailureText(
      new APIRequestError({
        code: "upstream_error",
        message: "Overpass API request failed with status 429",
        hint: "The Overpass server is busy or the query timed out. Wait a moment, then try again with a smaller bounding box.",
      }),
    );

    expect(text).toContain("status 429");
    expect(text).toContain("smaller bounding box");
  });

  it("falls back to the message when the envelope carries no hint", () => {
    const text = osmFailureText(
      new APIRequestError({
        code: "upstream_error",
        message: "Overpass API request failed",
      }),
    );

    expect(text).toBe("Overpass API request failed");
  });

  it("handles a plain error, which carries no envelope at all", () => {
    expect(osmFailureText(new Error("network down"))).toBe("network down");
  });
});

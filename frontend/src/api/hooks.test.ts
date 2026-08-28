import { describe, expect, it } from "vitest";
import type { BrowserRunSpec } from "./browser-backend";
import { buildCreateRunRequest } from "./hooks";

const baseSpec: BrowserRunSpec = {
  standardId: "rls19-road",
  version: "2019",
  profile: "default",
  params: {},
  receiverMode: "auto-grid",
};

/**
 * The request body is the contract the API gates on: a scaffold-tier standard
 * is refused unless `experimental` is present, and a standard that needs no
 * acknowledgement must not carry one.
 */
describe("buildCreateRunRequest", () => {
  it("carries the acknowledgement for a scaffold-tier run", () => {
    const request = buildCreateRunRequest({
      ...baseSpec,
      standardId: "cnossos-road",
      experimental: true,
    });

    expect(request.experimental).toBe(true);
    expect(request.standard_id).toBe("cnossos-road");
  });

  it("omits the field entirely when nothing was acknowledged", () => {
    const request = buildCreateRunRequest(baseSpec);

    expect(request).not.toHaveProperty("experimental");
    expect(Object.keys(request)).not.toContain("experimental");
  });

  it("omits the field rather than sending false", () => {
    const request = buildCreateRunRequest({
      ...baseSpec,
      experimental: false,
    });

    expect(request).not.toHaveProperty("experimental");
  });

  it("maps the rest of the spec onto the API field names", () => {
    const request = buildCreateRunRequest({
      ...baseSpec,
      profile: "strict",
      params: { temperature_c: "10" },
      receiverMode: "custom",
    });

    expect(request).toMatchObject({
      standard_id: "rls19-road",
      standard_version: "2019",
      standard_profile: "strict",
      receiver_mode: "custom",
      params: { temperature_c: "10" },
    });
  });
});

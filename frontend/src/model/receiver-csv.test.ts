/**
 * Unit cases for the receiver-CSV builder.
 *
 * The byte contract itself is pinned against Go in receiver-csv.parity.test.ts,
 * which replays a fixture the CLI wrote. What lives here is everything that
 * fixture cannot reach, in two groups:
 *
 *   - states `ReceiverTable.Validate` refuses on the Go side, so no Go golden
 *     can ever contain them — an empty field, an empty record list, a missing
 *     indicator, NaN, ±Infinity;
 *   - the shortest-digit property over a few thousand pseudo-random doubles,
 *     which is the half of the spelling risk no finite fixture covers.
 */

import { describe, expect, it } from "vitest";

import {
  buildReceiverTableCSV,
  encodeCSVField,
  formatCSVFloat,
} from "./receiver-csv";

describe("encodeCSVField", () => {
  it("writes an empty field as zero bytes, not as a quoted empty string", () => {
    // Go dropped quoting of the empty string in 1.4: Postgres distinguishes
    // the two on import, and `""` is the strictly less useful of them.
    expect(encodeCSVField("")).toBe("");
  });

  it("quotes the Postgres end-of-data sentinel", () => {
    expect(encodeCSVField("\\.")).toBe('"\\."');
    // ...but only when the field is exactly that, not when it merely contains it.
    expect(encodeCSVField("a\\.b")).toBe("a\\.b");
  });

  it("doubles an embedded quote", () => {
    expect(encodeCSVField('R"3')).toBe('"R""3"');
    expect(encodeCSVField('"')).toBe('""""');
  });

  it("quotes a comma, a CR and an LF and copies them verbatim", () => {
    expect(encodeCSVField("R,2")).toBe('"R,2"');
    expect(encodeCSVField("a\nb")).toBe('"a\nb"');
    // UseCRLF is false on the Go side, so a lone CR stays a lone CR.
    expect(encodeCSVField("a\rb")).toBe('"a\rb"');
  });

  it("quotes on a leading Go space but never on a trailing one", () => {
    expect(encodeCSVField(" R4")).toBe('" R4"');
    expect(encodeCSVField("R5 ")).toBe("R5 ");
    expect(encodeCSVField("a b")).toBe("a b");
  });

  it("follows Go's unicode.IsSpace, not JavaScript's \\s", () => {
    // U+0085 NEL is a Go space; JS `\s` does not match it.
    expect(encodeCSVField("\u0085R10")).toBe('"\u0085R10"');
    // U+FEFF is a JS `\s`; Go's unicode.IsSpace does not match it.
    expect(encodeCSVField("\ufeffR11")).toBe("\ufeffR11");
    // The rest of the set both agree on.
    expect(encodeCSVField("\u00a0x")).toBe('"\u00a0x"');
    expect(encodeCSVField("\u1680x")).toBe('"\u1680x"');
    expect(encodeCSVField("\u2028x")).toBe('"\u2028x"');
    expect(encodeCSVField("\u3000x")).toBe('"\u3000x"');
  });

  it("does not treat a non-space code point above the BMP as a space", () => {
    expect(encodeCSVField("\u{1d400}x")).toBe("\u{1d400}x");
  });
});

describe("formatCSVFloat", () => {
  it("keeps the sign on negative zero", () => {
    // String(-0) is "0"; strconv.FormatFloat(-0, 'f', -1, 64) is "-0".
    expect(formatCSVFloat(-0)).toBe("-0");
    expect(formatCSVFloat(0)).toBe("0");
  });

  it("spells the non-finite values the way Go does", () => {
    expect(formatCSVFloat(NaN)).toBe("NaN");
    expect(formatCSVFloat(Infinity)).toBe("+Inf");
    expect(formatCSVFloat(-Infinity)).toBe("-Inf");
  });

  it("expands the large end, where String() switches to an exponent", () => {
    expect(formatCSVFloat(1e20)).toBe("100000000000000000000");
    expect(formatCSVFloat(1e21)).toBe("1000000000000000000000");
    expect(formatCSVFloat(-1e21)).toBe("-1000000000000000000000");
    expect(formatCSVFloat(1.5e21)).toBe("1500000000000000000000");
    expect(formatCSVFloat(Number.MAX_VALUE)).toBe(
      `17976931348623157${"0".repeat(292)}`,
    );
  });

  it("expands the small end, where String() switches to an exponent", () => {
    expect(formatCSVFloat(1e-6)).toBe("0.000001");
    expect(formatCSVFloat(1e-7)).toBe("0.0000001");
    expect(formatCSVFloat(-1e-7)).toBe("-0.0000001");
    expect(formatCSVFloat(1.25e-7)).toBe("0.000000125");
    expect(formatCSVFloat(5e-324)).toBe(`0.${"0".repeat(323)}5`);
  });

  it("leaves an already positional spelling alone", () => {
    expect(formatCSVFloat(100.5)).toBe("100.5");
    expect(formatCSVFloat(0.30000000000000004)).toBe("0.30000000000000004");
    expect(formatCSVFloat(-100.5)).toBe("-100.5");
  });

  /**
   * The property a fixture cannot state: over a wide spread of doubles the
   * output never carries an exponent, and it parses back to exactly the value
   * it came from. Object.is rather than === so -0 is actually checked.
   *
   * Seeded xorshift64 so a failure is reproducible; the doubles are built from
   * raw bit patterns rather than from arithmetic, so the sample reaches
   * subnormals and the top of the exponent range as readily as it reaches 1.0.
   */
  it("round-trips a seeded sample of doubles without ever emitting an exponent", () => {
    const view = new DataView(new ArrayBuffer(8));
    let state = 0x2545f4914f6cdd1dn;
    const mask = (1n << 64n) - 1n;

    const next = (): bigint => {
      state ^= (state << 13n) & mask;
      state ^= state >> 7n;
      state ^= (state << 17n) & mask;
      return state;
    };

    let checked = 0;
    for (let i = 0; i < 4000; i += 1) {
      view.setBigUint64(0, next());
      const value = view.getFloat64(0);
      // NaN and ±Infinity have their own spelling and no round trip.
      if (!Number.isFinite(value)) continue;

      const text = formatCSVFloat(value);

      expect(text).not.toContain("e");
      expect(text).not.toContain("E");
      expect(Object.is(Number(text), value)).toBe(true);
      checked += 1;
    }

    // A sanity floor on the sample itself: a generator that produced only
    // non-finite values would pass every assertion above vacuously.
    expect(checked).toBeGreaterThan(3000);
  });
});

describe("buildReceiverTableCSV", () => {
  it("terminates every record with an LF, the last one included", () => {
    const csv = buildReceiverTableCSV({
      indicator_order: ["Lden"],
      records: [{ id: "R1", x: 1, y: 2, height_m: 3, values: { Lden: 60 } }],
    });

    expect(csv).toBe("id,x,y,height_m,Lden\nR1,1,2,3,60\n");
    expect(csv).not.toContain("\r");
  });

  it("writes a header and a newline for an empty record list", () => {
    expect(
      buildReceiverTableCSV({ indicator_order: ["Lden"], records: [] }),
    ).toBe("id,x,y,height_m,Lden\n");
  });

  it("takes the header from indicator_order, not from the records", () => {
    const csv = buildReceiverTableCSV({
      indicator_order: ["Lnight", "Lden"],
      records: [
        { id: "R1", x: 1, y: 2, height_m: 3, values: { Lden: 60, Lnight: 50 } },
      ],
    });

    expect(csv).toBe("id,x,y,height_m,Lnight,Lden\nR1,1,2,3,50,60\n");
  });

  it("quotes an indicator name that needs it", () => {
    const csv = buildReceiverTableCSV({
      indicator_order: ["Lr,Night"],
      records: [
        { id: "R1", x: 1, y: 2, height_m: 3, values: { "Lr,Night": 5 } },
      ],
    });

    expect(csv).toBe('id,x,y,height_m,"Lr,Night"\nR1,1,2,3,5\n');
  });

  it("leaves a missing indicator empty rather than writing a 0", () => {
    // Unreachable from Go — ReceiverTable.Validate rejects a record missing an
    // ordered indicator — but reachable in the browser, and an empty field
    // keeps "not measured" distinguishable from "measured zero".
    const csv = buildReceiverTableCSV({
      indicator_order: ["Lden", "Lnight"],
      records: [
        { id: "R10", x: 300, y: 100, height_m: 8, values: { Lden: 71.8 } },
      ],
    });

    expect(csv).toBe("id,x,y,height_m,Lden,Lnight\nR10,300,100,8,71.8,\n");
  });

  it("writes an empty id as zero bytes", () => {
    // Also unreachable from Go: Validate requires a non-empty id.
    const csv = buildReceiverTableCSV({
      indicator_order: ["Lden"],
      records: [{ id: "", x: 1, y: 2, height_m: 3, values: { Lden: 60 } }],
    });

    expect(csv).toBe("id,x,y,height_m,Lden\n,1,2,3,60\n");
  });

  it("escapes a quote in an id as a doubled quote", () => {
    const csv = buildReceiverTableCSV({
      indicator_order: ["Lden"],
      records: [{ id: 'R"1', x: 1, y: 2, height_m: 3, values: { Lden: 60 } }],
    });

    expect(csv).toBe('id,x,y,height_m,Lden\n"R""1",1,2,3,60\n');
  });

  it("quotes an id containing a comma", () => {
    const csv = buildReceiverTableCSV({
      indicator_order: ["Lden"],
      records: [{ id: "R,2", x: 1, y: 2, height_m: 3, values: { Lden: 60 } }],
    });

    expect(csv).toBe('id,x,y,height_m,Lden\n"R,2",1,2,3,60\n');
  });

  it("carries the non-finite spellings through into a record", () => {
    // Validate rejects all three on the Go side, and a negative height too.
    const csv = buildReceiverTableCSV({
      indicator_order: ["Lden", "Lnight"],
      records: [
        {
          id: "R1",
          x: NaN,
          y: Infinity,
          height_m: -1,
          values: { Lden: -Infinity, Lnight: -0 },
        },
      ],
    });

    expect(csv).toBe("id,x,y,height_m,Lden,Lnight\nR1,NaN,+Inf,-1,-Inf,-0\n");
  });
});

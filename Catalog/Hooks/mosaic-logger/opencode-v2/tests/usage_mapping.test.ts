/**
 * Mapping of a V2 tokens value to the MosaicLogFormat token_usage shape.
 */

import { describe, expect, it } from "vitest";

import { tokenUsageFromV2 } from "../lib/usage";

describe("tokenUsageFromV2", () => {
  it("adds reasoning into output and maps cache read and write to the cache fields", () => {
    const usage = tokenUsageFromV2({ input: 100, output: 20, reasoning: 5, cache: { read: 7, write: 3 } });

    expect(usage).toEqual({
      input_tokens: 100,
      output_tokens: 25,
      reasoning_tokens: 5,
      cache_read_tokens: 7,
      cache_creation_tokens: 3,
    });
  });

  it("keeps input as reported, because V2 input excludes cache", () => {
    const usage = tokenUsageFromV2({ input: 6997, output: 152, reasoning: 0, cache: { read: 4000, write: 0 } });

    expect(usage?.input_tokens).toBe(6997);
    expect(usage?.cache_read_tokens).toBe(4000);
  });

  it("sets zero values, which are reported values", () => {
    const usage = tokenUsageFromV2({ input: 0, output: 0, reasoning: 0, cache: { read: 0, write: 0 } });

    expect(usage).toEqual({
      input_tokens: 0,
      output_tokens: 0,
      reasoning_tokens: 0,
      cache_read_tokens: 0,
      cache_creation_tokens: 0,
    });
  });

  it("uses output alone when reasoning is not a number, and sets no reasoning field", () => {
    const usage = tokenUsageFromV2({ input: 10, output: 20, reasoning: "many" });

    expect(usage?.output_tokens).toBe(20);
    expect(usage && "reasoning_tokens" in usage).toBe(false);
  });

  it("sets no output field when output is not a number, even with reasoning present", () => {
    const usage = tokenUsageFromV2({ input: 10, reasoning: 5 });

    expect(usage?.input_tokens).toBe(10);
    expect(usage && "output_tokens" in usage).toBe(false);
    expect(usage?.reasoning_tokens).toBe(5);
  });

  it("maps only the cache sub-fields that are numbers", () => {
    const usage = tokenUsageFromV2({ input: 1, output: 1, cache: { read: 9 } });

    expect(usage?.cache_read_tokens).toBe(9);
    expect(usage && "cache_creation_tokens" in usage).toBe(false);
  });

  it("ignores cost and every unknown key", () => {
    const usage = tokenUsageFromV2({ input: 1, output: 2, cost: 3.5, total: 99, extra: { a: 1 } });

    expect(usage).toEqual({ input_tokens: 1, output_tokens: 2 });
    expect(JSON.stringify(usage)).not.toContain("cost");
  });

  it.each([undefined, null, 5, "tokens", [], {}, { cost: 1 }, { input: "5" }, { input: Number.NaN }, { output: Number.POSITIVE_INFINITY }, { cache: {} }])(
    "returns undefined when no sub-field is set from %j",
    (value) => {
      expect(tokenUsageFromV2(value)).toBeUndefined();
    },
  );
});

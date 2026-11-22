/**
 * usage.ts - token usage mapping from V2 reports to the MosaicLogFormat token_usage shape.
 *
 * V2 `tokens.input` excludes cache and `tokens.output` excludes reasoning. The
 * format has no cost field, so V2's cost values are never read.
 */

import { get, rec } from "./safe.js";

export interface TokenUsage {
  input_tokens?: number;
  output_tokens?: number;
  reasoning_tokens?: number;
  cache_read_tokens?: number;
  cache_creation_tokens?: number;
}

/** The value when it is a genuine finite number (zero included), else undefined. */
function num(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

/**
 * Maps a V2 `tokens` value to MosaicLogFormat token_usage. Reasoning is added into
 * output_tokens and also reported as reasoning_tokens. undefined when no sub-field
 * is set. Pure; never throws.
 */
export function tokenUsageFromV2(tokens: unknown): TokenUsage | undefined {
  try {
    if (rec(tokens) === undefined) return undefined;
    const input = num(get(tokens, "input"));
    const output = num(get(tokens, "output"));
    const reasoning = num(get(tokens, "reasoning"));
    const cacheRead = num(get(get(tokens, "cache"), "read"));
    const cacheWrite = num(get(get(tokens, "cache"), "write"));

    const usage: TokenUsage = {};
    if (input !== undefined) usage.input_tokens = input;
    if (output !== undefined) usage.output_tokens = output + (reasoning ?? 0);
    if (reasoning !== undefined) usage.reasoning_tokens = reasoning;
    if (cacheRead !== undefined) usage.cache_read_tokens = cacheRead;
    if (cacheWrite !== undefined) usage.cache_creation_tokens = cacheWrite;
    return Object.keys(usage).length === 0 ? undefined : usage;
  } catch {
    return undefined;
  }
}

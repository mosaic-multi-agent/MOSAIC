/**
 * Per-event schema version stamps: V1's table plus 1.4.0 on invocation_start.
 */

import { describe, expect, it } from "vitest";

import { schemaVersionFor } from "../lib/schema_versions";

describe("schemaVersionFor", () => {
  it.each([
    ["invocation_start", "1.4.0"],
    ["compaction", "1.5.0"],
    ["tool_call_end", "1.6.0"],
    ["usage_record", "1.7.0"],
  ])("stamps %s with %s", (event, version) => {
    expect(schemaVersionFor(event)).toBe(version);
  });

  it.each(["run_start", "run_end", "session_start", "session_end", "invocation_end", "tool_call_start", "turn", "notification"])(
    "stamps %s with the default 1.3.0",
    (event) => {
      expect(schemaVersionFor(event)).toBe("1.3.0");
    },
  );

  it("stamps an unknown event with the default", () => {
    expect(schemaVersionFor("something_new")).toBe("1.3.0");
    expect(schemaVersionFor("toString")).toBe("1.3.0");
  });
});

/**
 * manifest_version.test.ts -- Tests for reading adapter_version from the
 * workspace manifest (.mosaic/manifest.yaml).
 *
 * Manifest content is written to a temp directory, shaped like the file the
 * Deploy tool writes. No real workspace manifest is read.
 */

import { describe, it, expect, beforeEach, afterEach } from "vitest";
import * as nodePath from "node:path";
import * as nodeFs from "node:fs";
import * as nodeOs from "node:os";

import { setDebugLogger } from "../lib/core";
import { readManifestAdapterVersion } from "../lib/manifest_version";

interface EntryOpts {
  kind: string;
  key: string;
  version?: string; // raw value text, written verbatim; omitted when undefined
  catalogVersion?: string;
}

function entry(o: EntryOpts): string {
  const lines = [
    "- ref:",
    `    kind: ${o.kind}`,
    `    key: ${o.key}`,
    `  target_path: .opencode/plugins/${o.key}`,
  ];
  if (o.version !== undefined) lines.push(`  version: ${o.version}`);
  lines.push(
    "  harness_version: 1.2.3",
    "  injections_version: 4.5.6",
    "  content_hash: abc123",
    "  deployed_at: 2026-01-01T00:00:00Z",
  );
  if (o.catalogVersion !== undefined) {
    lines.push("  catalog:", `    version: ${o.catalogVersion}`, "    source: catalog");
  }
  return lines.join("\n");
}

const TRAILER = [
  "tool_version:",
  "  name: deploy",
  "  version: 9.9.9",
  "protocol_source:",
  "  path: protocol",
  "  version: 8.8.8",
  "bundle_source:",
  "  path: bundle",
  "  version: 7.7.7",
].join("\n");

function manifest(entries: string[], trailer: string = TRAILER): string {
  return [
    "schema_version: 1",
    "harness_id: opencode",
    "updated_at: 2026-01-01T00:00:00Z",
    "entries:",
    ...entries,
    trailer,
    "",
  ].join("\n");
}

const hook = (version?: string, extra: Partial<EntryOpts> = {}): string =>
  entry({ kind: "hook", key: "mosaic-logger", version, ...extra });

describe("readManifestAdapterVersion", () => {
  let root: string;
  let logs: string[];

  function write(content: string): void {
    nodeFs.mkdirSync(nodePath.join(root, ".mosaic"), { recursive: true });
    nodeFs.writeFileSync(nodePath.join(root, ".mosaic", "manifest.yaml"), content);
  }

  beforeEach(() => {
    root = nodeFs.mkdtempSync(nodePath.join(nodeOs.tmpdir(), "mosaic-manifest-version-"));
    logs = [];
    setDebugLogger((m) => {
      logs.push(m);
    });
  });

  afterEach(() => {
    setDebugLogger(() => {});
    nodeFs.rmSync(root, { recursive: true, force: true });
  });

  describe("version found", () => {
    it("returns the mosaic-logger hook entry version for a lone entry", async () => {
      write(manifest([hook("1.4.0")]));
      expect(await readManifestAdapterVersion(root)).toBe("1.4.0");
    });

    it("returns it when the entry is first, among other kinds and keys", async () => {
      write(
        manifest([
          hook("1.4.0"),
          entry({ kind: "hook", key: "other-hook", version: "2.0.0" }),
          entry({ kind: "agent", key: "mosaic-logger", version: "3.0.0" }),
        ]),
      );
      expect(await readManifestAdapterVersion(root)).toBe("1.4.0");
    });

    it("returns it when the entry is in the middle", async () => {
      write(
        manifest([
          entry({ kind: "hook", key: "other-hook", version: "2.0.0" }),
          hook("1.4.0"),
          entry({ kind: "agent", key: "mosaic-logger", version: "3.0.0" }),
        ]),
      );
      expect(await readManifestAdapterVersion(root)).toBe("1.4.0");
    });

    it("returns it when the entry is last", async () => {
      write(
        manifest([
          entry({ kind: "hook", key: "other-hook", version: "2.0.0" }),
          entry({ kind: "tool", key: "mosaic-logger", version: "3.0.0" }),
          hook("1.4.0"),
        ]),
      );
      expect(await readManifestAdapterVersion(root)).toBe("1.4.0");
    });

    it("ignores the nested catalog version", async () => {
      write(manifest([hook("1.4.0", { catalogVersion: "0.0.1" })]));
      expect(await readManifestAdapterVersion(root)).toBe("1.4.0");
    });

    it("strips one layer of double quotes", async () => {
      write(manifest([hook('"1.4.0"')]));
      expect(await readManifestAdapterVersion(root)).toBe("1.4.0");
    });

    it("strips one layer of single quotes", async () => {
      write(manifest([hook("'1.4.0'")]));
      expect(await readManifestAdapterVersion(root)).toBe("1.4.0");
    });

    it("handles CRLF line endings", async () => {
      write(manifest([hook("1.4.0", { catalogVersion: "0.0.1" })]).replace(/\n/g, "\r\n"));
      expect(await readManifestAdapterVersion(root)).toBe("1.4.0");
    });

    it("handles CRLF with a quoted value", async () => {
      write(manifest([hook('"1.4.0"')]).replace(/\n/g, "\r\n"));
      expect(await readManifestAdapterVersion(root)).toBe("1.4.0");
    });
  });

  describe("no version", () => {
    async function expectOmitted(): Promise<void> {
      logs.length = 0;
      await expect(readManifestAdapterVersion(root)).resolves.toBeUndefined();
      expect(logs.length).toBeGreaterThan(0);
    }

    it("is omitted, with a debug line, when the manifest is missing", async () => {
      await expectOmitted();
    });

    it("is omitted when .mosaic/manifest.yaml is a directory (unreadable)", async () => {
      nodeFs.mkdirSync(nodePath.join(root, ".mosaic", "manifest.yaml"), { recursive: true });
      await expectOmitted();
    });

    it("is omitted when there is no mosaic-logger hook entry", async () => {
      write(
        manifest([
          entry({ kind: "hook", key: "other-hook", version: "2.0.0" }),
          entry({ kind: "agent", key: "mosaic-logger", version: "3.0.0" }),
        ]),
      );
      await expectOmitted();
    });

    it("is omitted when there are no entries at all", async () => {
      write(manifest([]));
      await expectOmitted();
    });

    it("never takes a trailing block version when the last entry has no version", async () => {
      write(
        manifest([
          entry({ kind: "hook", key: "other-hook", version: "2.0.0" }),
          hook(undefined),
        ]),
      );
      await expectOmitted();
    });

    it("never takes a trailing block version at end of file without other keys", async () => {
      write(manifest([hook(undefined)], "protocol_source:\n  version: 8.8.8"));
      await expectOmitted();
    });

    it("never takes the catalog version when the entry has no version", async () => {
      write(manifest([hook(undefined, { catalogVersion: "0.0.1" })]));
      await expectOmitted();
    });

    it("never takes a following entry's version when the entry has none", async () => {
      write(
        manifest([hook(undefined), entry({ kind: "agent", key: "x", version: "5.0.0" })]),
      );
      await expectOmitted();
    });

    it("is omitted for duplicate matching entries", async () => {
      write(manifest([hook("1.4.0"), hook("1.5.0")]));
      await expectOmitted();
    });

    it("is omitted for duplicate entry-level version lines", async () => {
      const dup = hook("1.4.0").replace(
        "  harness_version: 1.2.3",
        "  version: 1.5.0\n  harness_version: 1.2.3",
      );
      write(manifest([dup]));
      await expectOmitted();
    });

    it("is omitted for an empty version", async () => {
      write(manifest([hook("")]));
      await expectOmitted();
    });

    it("is omitted for an empty quoted version", async () => {
      write(manifest([hook('""')]));
      await expectOmitted();
    });

    it("is omitted for an unparseable (non-scalar) version", async () => {
      write(manifest([hook("{nested: value}")]));
      await expectOmitted();
    });
  });

  it("does not read hook.yaml next to the plugin", async () => {
    nodeFs.mkdirSync(nodePath.join(root, ".opencode", "plugins"), { recursive: true });
    nodeFs.writeFileSync(
      nodePath.join(root, ".opencode", "plugins", "hook.yaml"),
      'version: "9.9.9"\n',
    );
    await expect(readManifestAdapterVersion(root)).resolves.toBeUndefined();
  });
});

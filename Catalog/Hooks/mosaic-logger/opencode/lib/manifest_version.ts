/**
 * manifest_version.ts -- Read the mosaic-logger hook version from the workspace
 * manifest (<workspace root>/.mosaic/manifest.yaml) without a YAML parser.
 */

import * as nodePath from "node:path";
import * as nodeFsPromises from "node:fs/promises";

import { debugLog } from "./core.js";

const ENTRY_START_RE = /^-\s+ref:\s*$/;
const KIND_RE = /^ {4}kind:\s*(.*?)\s*$/;
const KEY_RE = /^ {4}key:\s*(.*?)\s*$/;
const VERSION_RE = /^ {2}version:\s*(.*?)\s*$/;

function unquote(value: string): string {
  if (
    value.length >= 2 &&
    ((value[0] === '"' && value[value.length - 1] === '"') ||
      (value[0] === "'" && value[value.length - 1] === "'"))
  ) {
    return value.slice(1, -1);
  }
  return value;
}

interface ManifestEntry {
  kind?: string;
  key?: string;
  versions: string[];
}

/** Split manifest text into entries; an entry ends at the next column-0 line. */
function parseEntries(content: string): ManifestEntry[] {
  const entries: ManifestEntry[] = [];
  let current: ManifestEntry | undefined;

  for (const rawLine of content.split("\n")) {
    const line = rawLine.replace(/\r$/, "");
    if (!line.trim()) continue;

    if (line[0] !== " " && line[0] !== "\t") {
      // Column 0: a new entry or a top-level key (ends the current entry)
      current = ENTRY_START_RE.test(line) ? { versions: [] } : undefined;
      if (current) entries.push(current);
      continue;
    }
    if (!current) continue;

    const kind = KIND_RE.exec(line);
    if (kind) {
      current.kind = unquote(kind[1]);
      continue;
    }
    const key = KEY_RE.exec(line);
    if (key) {
      current.key = unquote(key[1]);
      continue;
    }
    const version = VERSION_RE.exec(line);
    if (version) current.versions.push(unquote(version[1]));
  }
  return entries;
}

/**
 * Return the version of the manifest entry with kind "hook" and key
 * "mosaic-logger". Returns undefined (with a debug log line) when the manifest
 * is unreadable, the entry is missing or ambiguous, or the version is empty or
 * unparseable. Never throws.
 */
export async function readManifestAdapterVersion(
  workspaceRoot: string,
): Promise<string | undefined> {
  try {
    const manifestPath = nodePath.join(workspaceRoot, ".mosaic", "manifest.yaml");
    let content: string;
    try {
      content = await nodeFsPromises.readFile(manifestPath, "utf-8");
    } catch (err) {
      debugLog("adapter_version omitted: manifest unreadable", err);
      return undefined;
    }

    const matches = parseEntries(content).filter(
      (e) => e.kind === "hook" && e.key === "mosaic-logger",
    );
    if (matches.length !== 1) {
      debugLog(
        `adapter_version omitted: ${matches.length} mosaic-logger hook entries in manifest`,
      );
      return undefined;
    }
    const { versions } = matches[0];
    if (versions.length !== 1) {
      debugLog(
        `adapter_version omitted: ${versions.length} version values in mosaic-logger entry`,
      );
      return undefined;
    }
    const value = versions[0];
    if (!value || value.includes("{") || value.includes("[")) {
      debugLog("adapter_version omitted: version empty or unparseable");
      return undefined;
    }
    return value;
  } catch (err) {
    debugLog("adapter_version omitted: unexpected error reading manifest", err);
    return undefined;
  }
}

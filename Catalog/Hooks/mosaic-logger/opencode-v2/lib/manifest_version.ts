/**
 * manifest_version.ts - read the mosaic-logger hook version from the workspace
 * manifest (<workspace root>/.mosaic/manifest.yaml) without a YAML parser.
 */

import * as nodeFsPromises from "node:fs/promises";
import * as nodePath from "node:path";

const ENTRY_START_RE = /^-\s+ref:\s*$/;
const KIND_RE = /^ {4}kind:\s*(.*?)\s*$/;
const KEY_RE = /^ {4}key:\s*(.*?)\s*$/;
const VERSION_RE = /^ {2}version:\s*(.*?)\s*$/;

function unquote(value: string): string {
  const quoted =
    value.length >= 2 &&
    ((value[0] === '"' && value[value.length - 1] === '"') ||
      (value[0] === "'" && value[value.length - 1] === "'"));
  return quoted ? value.slice(1, -1) : value;
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

/** The version of the single manifest entry with kind hook and key mosaic-logger, else undefined. */
export function parseAdapterVersion(content: string): string | undefined {
  const matches = parseEntries(content).filter((e) => e.kind === "hook" && e.key === "mosaic-logger");
  if (matches.length !== 1) return undefined;
  const { versions } = matches[0];
  if (versions.length !== 1) return undefined;
  const value = versions[0];
  if (!value || value.includes("{") || value.includes("[")) return undefined;
  return value;
}

/**
 * Read the adapter version from the workspace manifest. Resolves undefined when
 * the manifest is unreadable, the entry is missing or ambiguous, or the version
 * is empty or unparseable. Never rejects. The caller must not await it on a
 * path that has to stay fast.
 */
export async function readManifestAdapterVersion(workspaceRoot: string): Promise<string | undefined> {
  try {
    const manifestPath = nodePath.join(workspaceRoot, ".mosaic", "manifest.yaml");
    return parseAdapterVersion(await nodeFsPromises.readFile(manifestPath, "utf-8"));
  } catch {
    return undefined;
  }
}

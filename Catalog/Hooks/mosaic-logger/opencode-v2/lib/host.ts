/**
 * host.ts - the subset of the OpenCode V2 plugin context the logger uses.
 *
 * Types only. Every field is optional and loosely typed because the plugin reads
 * all of them defensively; test fakes implement this shape.
 */

export interface V2EventLocation {
  directory?: string;
  workspaceID?: string;
}

export interface V2Event {
  id?: string;
  created?: number;
  type?: string;
  location?: V2EventLocation;
  data?: Record<string, unknown>;
}

export interface V2ToolHookEvent {
  tool?: string;
  sessionID?: string;
  agent?: string;
  messageID?: string;
  /** Tool call id. */
  id?: string;
  input?: unknown;
  /** execute.after only: "completed" | "error". */
  status?: string;
  /** execute.after, completed. */
  result?: unknown;
  /** execute.after, error. */
  error?: unknown;
}

export interface V2PermissionEvaluateEvent {
  sessionID?: string;
  agent?: string;
  action?: string;
  resources?: unknown;
  metadata?: unknown;
  source?: unknown;
  effect?: string;
  message?: string;
}

type Registration = unknown | Promise<unknown>;

export interface V2PluginContext {
  location?: {
    directory?: string;
    workspaceID?: string;
    project?: { id?: string; directory?: string; canonical?: string };
  };
  tool?: {
    hook?: (
      name: "execute.before" | "execute.after",
      cb: (event: V2ToolHookEvent) => unknown,
    ) => Registration;
  };
  permission?: {
    hook?: (name: "evaluate", cb: (event: V2PermissionEvaluateEvent) => unknown) => Registration;
  };
  session?: {
    hook?: (name: string, cb: (event: Record<string, unknown>) => unknown) => Registration;
  };
  event?: {
    subscribe?: (options: { types?: string[]; signal?: AbortSignal }) => AsyncIterable<V2Event>;
  };
}

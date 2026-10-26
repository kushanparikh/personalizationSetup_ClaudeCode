import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import type { Answers, HookType, PermId, PermStatus, Report, ServeStatus } from "./types";

export class ApiError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ApiError";
  }
}

/** Tauri rejects with a bare string; everything past `call` is an ApiError. */
export function errorText(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

async function call<T>(cmd: string, args?: Record<string, unknown>): Promise<T> {
  try {
    return await invoke<T>(cmd, args);
  } catch (e) {
    throw new ApiError(errorText(e));
  }
}

export type SendTestResult = { sent: true } | { preview: string };

export const api = {
  state: () => call<Report>("get_state"),
  apply: (hookType: HookType, answers: Answers) => call<Report>("apply_hook", { hookType, answers }),
  enable: (hookType: HookType) => call<Report>("enable_hook", { hookType }),
  disable: (hookType: HookType) => call<Report>("disable_hook", { hookType }),
  remove: (hookType: HookType) => call<Report>("remove_hook", { hookType }),
  requestPermission: (id: PermId) => call<{ status: PermStatus }>("request_permission", { id }),
  sendTest: (hookType: HookType) => call<SendTestResult>("send_test", { hookType }),
  openSettings: (url: string) => call<void>("open_settings", { url }),
  openLog: () => call<void>("open_log"),
  serveStatus: () => call<ServeStatus>("serve_status"),
};

export function onServeStatus(cb: (s: ServeStatus) => void): Promise<() => void> {
  return listen<ServeStatus>("serve-status", (e) => cb(e.payload));
}

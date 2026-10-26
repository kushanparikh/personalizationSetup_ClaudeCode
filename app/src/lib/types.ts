export type HookType = "git-status" | "finished" | "input-needed";
export type PermId = "automation-terminal";
export type PermStatus = "granted" | "denied" | "not-asked" | "unknown";
export type HookStatus = "ok" | "pending" | "off";

export interface Answers {
  untrackedDirty: boolean;
  sound: string;
  clickToFocus: boolean;
}

export interface HookDef {
  type: HookType;
  name: string;
  description: string;
  defaults: Answers;
}

export interface HookView {
  type: HookType;
  name: string;
  description: string;
  enabled: boolean;
  answers: Answers;
  status: HookStatus;
  needs: { id: PermId; status: PermStatus }[];
}

export interface OtherHook {
  event: string;
  matcher: string;
  command: string;
}

export interface PendingItem {
  id: PermId;
  title: string;
  steps: string;
  settingsUrl: string;
  status: PermStatus;
  affects: string[];
}

export interface Report {
  hooks: HookView[];
  available: HookDef[];
  otherHooks: OtherHook[];
  pending: PendingItem[];
  settingsError: string;
  logPath: string;
}

export type ServeStatus =
  | { state: "starting" }
  | { state: "ready" }
  | { state: "down"; reason: string };

export const HOOK_STATUS_TEXT: Record<HookStatus, string> = {
  ok: "Working",
  pending: "Waiting on permission",
  off: "Off",
};

export const SOUNDS = [
  "none", "Basso", "Blow", "Bottle", "Frog", "Funk", "Glass", "Hero",
  "Morse", "Ping", "Pop", "Purr", "Sosumi", "Submarine", "Tink",
] as const;

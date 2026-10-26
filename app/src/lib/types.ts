export type HookType = string;
export type PermId = string;
export type PermStatus = "granted" | "denied" | "not-asked" | "unknown";
export type HookStatus = "ok" | "pending" | "off";

export interface Answers {
  untrackedDirty: boolean;
  sound: string;
  clickToFocus: boolean;
}

/** One thing the settings form asks, described by pcs. `key` is an Answers field; `kind` picks the control. */
export interface Question {
  key: keyof Answers;
  kind: "bool" | "sound";
  label: string;
}

export interface HookDef {
  type: HookType;
  name: string;
  description: string;
  defaults: Answers;
  questions: Question[];
  note?: string;
}

export interface HookView {
  type: HookType;
  name: string;
  description: string;
  questions: Question[];
  note?: string;
  enabled: boolean;
  answers: Answers;
  status: HookStatus;
  needs: { id: PermId; title: string; status: PermStatus }[];
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
  askable: boolean;
  unknownHint: string;
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

/** What an alert window renders. `kind` is behaviour (panel = sticky, toast = timed); `tone` is look (lamp colour). */
export type AlertView = {
  kind: "panel" | "toast";
  /** "wait" | "done" | "danger"; the Rust side defaults it by kind. Unknown tones render as wait. */
  tone: string;
  title: string;
  message: string;
  focus: boolean;
};

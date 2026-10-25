import { api } from "./api";
import type { Answers, HookType, PermId, Report, ServeStatus } from "./types";

export type Selection =
  | { kind: "hook"; type: HookType }
  | { kind: "add"; type: HookType | null }
  | { kind: "pending"; id: PermId }
  | { kind: "other" }
  | { kind: "none" };

export class AppStore {
  report = $state<Report | null>(null);
  selection = $state<Selection>({ kind: "none" });
  busy = $state(false);
  error = $state<string | null>(null);
  notice = $state<string | null>(null);
  serve = $state<ServeStatus>({ state: "starting" });

  get canWrite(): boolean {
    return !this.busy && this.report !== null && this.report.settingsError === "";
  }

  /** Writes are refused while busy or when settings.json is invalid (a null report is allowed: the backend decides). */
  private get blocked(): boolean {
    return this.busy || (this.report !== null && this.report.settingsError !== "");
  }

  private async run(fn: () => Promise<void>): Promise<void> {
    this.busy = true;
    this.error = null;
    this.notice = null;
    try {
      await fn();
    } catch (e) {
      this.error = e instanceof Error ? e.message : String(e);
    } finally {
      this.busy = false;
    }
  }

  refresh(): Promise<void> {
    return this.run(async () => {
      this.report = await api.state();
    });
  }

  saveAndApply(type: HookType, answers: Answers): Promise<void> {
    if (this.blocked) return Promise.resolve();
    return this.run(async () => {
      let report = await api.apply(type, answers);
      this.report = report;
      this.selection = { kind: "hook", type };
      const hook = report.hooks.find((h) => h.type === type);
      let asked = false;
      for (const need of hook?.needs ?? []) {
        if (need.status === "not-asked" || need.status === "unknown") {
          await api.requestPermission(need.id);
          asked = true;
        }
      }
      if (asked) {
        report = await api.state();
        this.report = report;
      }
      const after = report.hooks.find((h) => h.type === type);
      const name = after?.name ?? type;
      if (after?.status === "ok") {
        this.notice = `✅ Saved. ${name} is set up.`;
      } else {
        const titles = (after?.needs ?? [])
          .filter((n) => n.status !== "granted")
          .map((n) => report.pending.find((p) => p.id === n.id)?.title ?? n.id);
        this.notice = `⚠️ Saved. Waiting on: ${titles.join(", ")} (see Pending).`;
      }
    });
  }

  setEnabled(type: HookType, on: boolean): Promise<void> {
    if (this.blocked) return Promise.resolve();
    return this.run(async () => {
      this.report = on ? await api.enable(type) : await api.disable(type);
    });
  }

  remove(type: HookType): Promise<void> {
    if (this.blocked) return Promise.resolve();
    return this.run(async () => {
      this.report = await api.remove(type);
      this.selection = { kind: "none" };
    });
  }

  sendTest(type: HookType): Promise<void> {
    return this.run(async () => {
      const r = await api.sendTest(type);
      this.notice = "preview" in r ? r.preview : "Sent. Look at the top-right of your screen.";
    });
  }

  openSettings(url: string): Promise<void> {
    return this.run(() => api.openSettings(url));
  }

  openLog(): Promise<void> {
    return this.run(() => api.openLog());
  }
}

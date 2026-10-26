import { api, errorText } from "./api";
import type { Answers, HookType, PermId, Report, ServeStatus } from "./types";

export type Selection =
  | { kind: "hook"; type: HookType }
  | { kind: "add"; type: HookType | null }
  | { kind: "pending"; id: PermId }
  | { kind: "other" }
  | { kind: "uninstall" }
  | { kind: "none" };

export class AppStore {
  report = $state<Report | null>(null);
  selection = $state<Selection>({ kind: "none" });
  busy = $state(false);
  error = $state<string | null>(null);
  notice = $state<string | null>(null);
  uninstalled = $state(false);
  serve = $state<ServeStatus>({ state: "starting" });

  get canWrite(): boolean {
    return !this.busy && this.report !== null && this.report.settingsError === "";
  }

  private async run(fn: () => Promise<void>): Promise<void> {
    this.busy = true;
    this.error = null;
    this.notice = null;
    try {
      await fn();
    } catch (e) {
      this.error = errorText(e);
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
    if (!this.canWrite) return Promise.resolve();
    return this.run(async () => {
      let report = await api.apply(type, answers);
      this.report = report;
      this.selection = { kind: "hook", type };
      const hook = report.hooks.find((h) => h.type === type);
      let asked = false;
      for (const need of hook?.needs ?? []) {
        if (need.askable && (need.status === "not-asked" || need.status === "unknown")) {
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
          .map((n) => n.title);
        this.notice = `⚠️ Saved. Waiting on: ${titles.join(", ")} (see Pending).`;
      }
    });
  }

  setEnabled(type: HookType, on: boolean): Promise<void> {
    if (!this.canWrite) return Promise.resolve();
    return this.run(async () => {
      this.report = on ? await api.enable(type) : await api.disable(type);
    });
  }

  remove(type: HookType): Promise<void> {
    if (!this.canWrite) return Promise.resolve();
    const name = this.report?.hooks.find((h) => h.type === type)?.name ?? type;
    return this.run(async () => {
      this.report = await api.remove(type);
      this.selection = { kind: "none" };
      this.notice = `Removed ${name}. Your settings.json was backed up first.`;
    });
  }

  /** Removes the hooks, data and app. The backend then quits the app, so the window is about to close. */
  uninstall(): Promise<void> {
    if (!this.canWrite || this.uninstalled) return Promise.resolve();
    return this.run(async () => {
      const r = await api.uninstall();
      this.uninstalled = true;
      const n = r.removedHooks;
      this.notice = `Uninstalled. Removed ${n} ${n === 1 ? "hook" : "hooks"}. Closing…`;
    });
  }

  sendTest(type: HookType): Promise<void> {
    return this.run(async () => {
      const r = await api.sendTest(type);
      this.notice = "preview" in r ? r.preview : "Sent. Look at the top-right of your screen.";
    });
  }

  /** Shows the macOS prompt for a permission it has not asked about yet, then re-reads state. */
  askPermission(id: PermId): Promise<void> {
    return this.run(async () => {
      await api.requestPermission(id);
      this.report = await api.state();
    });
  }

  openSettings(url: string): Promise<void> {
    return this.run(() => api.openSettings(url));
  }
}

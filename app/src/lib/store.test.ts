import { describe, expect, it } from "vitest";
import { mockIPC } from "@tauri-apps/api/mocks";
import { AppStore } from "./store.svelte";
import type { PermStatus, Report } from "./types";

const answers = { untrackedDirty: false, sound: "Glass", clickToFocus: true };

function report(over: Partial<Report> = {}, perm: PermStatus = "not-asked", ok = false): Report {
  return {
    hooks: [
      {
        type: "finished",
        name: "Finished working",
        description: "",
        enabled: true,
        answers,
        status: ok ? "ok" : "pending",
        needs: [{ id: "automation-terminal", status: ok ? "granted" : perm }],
      },
    ],
    available: [],
    otherHooks: [],
    pending: [
      {
        id: "automation-terminal",
        title: "Control Terminal",
        steps: "",
        settingsUrl: "x",
        status: ok ? "granted" : perm,
        affects: ["Finished working"],
      },
    ],
    settingsError: "",
    logPath: "",
    ...over,
  };
}

function script(h: Record<string, (args: any) => unknown>) {
  const calls: string[] = [];
  mockIPC((cmd, args) => {
    calls.push(cmd);
    if (!h[cmd]) throw `unscripted ${cmd}`;
    const r = h[cmd](args);
    if (r instanceof Error) throw r.message;
    return r;
  });
  return calls;
}

describe("AppStore", () => {
  it("save_requests_permission_when_not_asked", async () => {
    const calls = script({
      apply_hook: () => report(),
      request_permission: () => ({ status: "granted" }),
      get_state: () => report({}, "granted", true),
    });
    const s = new AppStore();
    s.report = report();
    await s.saveAndApply("finished", answers);
    expect(calls).toEqual(["apply_hook", "request_permission", "get_state"]);
    expect(s.notice).toBe("✅ Saved. Finished working is set up.");
    expect(s.selection).toEqual({ kind: "hook", type: "finished" });
    expect(s.busy).toBe(false);
  });

  it("save_requests_permission_when_unknown", async () => {
    const calls = script({
      apply_hook: () => report({}, "unknown"),
      request_permission: () => ({ status: "granted" }),
      get_state: () => report({}, "granted", true),
    });
    const s = new AppStore();
    s.report = report();
    await s.saveAndApply("finished", answers);
    expect(calls).toEqual(["apply_hook", "request_permission", "get_state"]);
    expect(s.notice).toBe("✅ Saved. Finished working is set up.");
  });

  it("save_pending_when_denied", async () => {
    const calls = script({
      apply_hook: () => report({}, "denied"),
      get_state: () => report({}, "denied"),
    });
    const s = new AppStore();
    s.report = report();
    await s.saveAndApply("finished", answers);
    expect(calls).not.toContain("request_permission");
    expect(s.notice).toBe("⚠️ Saved. Waiting on: Control Terminal (see Pending).");
  });

  it("save_error_keeps_report", async () => {
    const orig = report({}, "granted", true);
    script({
      get_state: () => orig,
      apply_hook: () => new Error("settings changed on disk, try again"),
    });
    const s = new AppStore();
    await s.refresh();
    await s.saveAndApply("finished", answers);
    expect(s.error).toBe("settings changed on disk, try again");
    expect(s.report).toEqual(orig);
    expect(s.busy).toBe(false);
  });

  it("writes_refused_when_settings_invalid", async () => {
    const calls = script({ get_state: () => report({ settingsError: "bad json" }) });
    const s = new AppStore();
    await s.refresh();
    expect(s.canWrite).toBe(false);
    await s.saveAndApply("finished", answers);
    expect(calls).not.toContain("apply_hook");
  });

  it("remove_clears_selection", async () => {
    script({ remove_hook: () => report({ hooks: [] }) });
    const s = new AppStore();
    s.report = report();
    s.selection = { kind: "hook", type: "finished" };
    await s.remove("finished");
    expect(s.selection).toEqual({ kind: "none" });
    expect(s.report?.hooks).toEqual([]);
  });

  it("send_test_preview_notice", async () => {
    script({ send_test: () => ({ preview: "📁 repo  🌿 main  ✅ clean" }) });
    const s = new AppStore();
    await s.sendTest("git-status");
    expect(s.notice).toBe("📁 repo  🌿 main  ✅ clean");
  });

  it("send_test_sent_notice", async () => {
    script({ send_test: () => ({ sent: true }) });
    const s = new AppStore();
    await s.sendTest("finished");
    expect(s.notice).toBe("Sent. Look at the top-right of your screen.");
  });
});

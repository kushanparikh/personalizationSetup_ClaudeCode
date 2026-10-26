import { describe, expect, it } from "vitest";
import { render, screen, fireEvent, waitFor, within } from "@testing-library/svelte";
import { mockIPC } from "@tauri-apps/api/mocks";
import { emit } from "@tauri-apps/api/event";
import App from "./App.svelte";
import { HOOK_STATUS_TEXT, type HookView, type Question, type Report } from "./lib/types";
import golden from "../../sidecar/internal/report/testdata/report.golden.json";

// Maps string-literal unions to string, which is all JSON import types give us.
type Widen<T> = T extends string
  ? string
  : T extends (infer U)[]
    ? Widen<U>[]
    : T extends object
      ? { [K in keyof T]: Widen<T[K]> }
      : T;
// Fails `npm run check` if the Go golden stops fitting `Report` (e.g. steps: string[]).
const _goldenFitsReport: Widen<Report> = golden;
void _goldenFitsReport;

const SPACES_NOTE =
  "If that session's window is on another desktop or full-screen Space, macOS won't switch to it. The tab is selected and Terminal comes forward.";
const gitQs: Question[] = [{ key: "untrackedDirty", kind: "bool", label: "Count untracked files as dirty" }];
const notifyQs: Question[] = [
  { key: "sound", kind: "sound", label: "Sound" },
  { key: "clickToFocus", kind: "bool", label: "Click focuses the session's terminal tab" },
];

const gitStatus: HookView = {
  type: "git-status",
  name: "Git status on session start",
  description: "Shows the repo, branch and whether it's clean when a session starts.",
  questions: gitQs,
  enabled: true,
  answers: { untrackedDirty: true, sound: "Glass", clickToFocus: true },
  status: "ok",
  needs: [],
};

const finished: HookView = {
  type: "finished",
  name: "Finished working",
  description: "A banner when Claude finishes and waits for you.",
  questions: notifyQs,
  note: SPACES_NOTE,
  enabled: true,
  answers: { untrackedDirty: false, sound: "Glass", clickToFocus: true },
  status: "ok",
  needs: [{ id: "automation-terminal", title: "Control Terminal", status: "granted", askable: true }],
};

const inputNeeded: HookView = {
  type: "input-needed",
  name: "Input needed",
  description: "A panel that stays until you answer Claude.",
  questions: notifyQs,
  note: SPACES_NOTE,
  enabled: true,
  answers: { untrackedDirty: false, sound: "Ping", clickToFocus: true },
  status: "pending",
  needs: [{ id: "automation-terminal", title: "Control Terminal", status: "denied", askable: true }],
};

function fixture(over: Partial<Report> = {}): Report {
  return {
    hooks: [gitStatus, finished],
    available: [
      {
        type: "input-needed",
        name: "Input needed",
        description: "A panel that stays until you answer Claude.",
        defaults: { untrackedDirty: false, sound: "Ping", clickToFocus: true },
        questions: notifyQs,
        note: SPACES_NOTE,
      },
    ],
    otherHooks: [
      { event: "PreToolUse", matcher: "Bash", command: "echo one" },
      { event: "Stop", matcher: "", command: "echo two" },
      { event: "SessionStart", matcher: "", command: "echo three" },
    ],
    pending: [
      {
        id: "automation-terminal",
        title: "Control Terminal",
        steps: "Open System Settings → Privacy & Security → Automation → Personalization Setup, and turn on Terminal.",
        settingsUrl: "x-apple.systempreferences:com.apple.preference.security?Privacy_Automation",
        status: "denied",
        askable: true,
        unknownHint: "Couldn't check this yet. Terminal may not be running. Click Ask for permission to check again.",
        affects: ["Input needed"],
      },
    ],
    settingsError: "",
    logPath: "/tmp/app.log",
    ...over,
  };
}

type Call = { cmd: string; args: any };

function setup(rep: Report, handlers: Record<string, (args: any) => unknown> = {}) {
  const calls: Call[] = [];
  mockIPC(
    (cmd, args) => {
      calls.push({ cmd, args });
      if (handlers[cmd]) return handlers[cmd](args);
      if (cmd === "get_state") return rep;
      if (cmd === "serve_status") return { state: "ready" };
      if (["apply_hook", "enable_hook", "disable_hook", "remove_hook"].includes(cmd)) return rep;
      return null;
    },
    { shouldMockEvents: true },
  );
  render(App);
  return calls;
}

const count = (calls: Call[], cmd: string) => calls.filter((c) => c.cmd === cmd).length;
const myHooks = () => screen.findByRole("list", { name: "My hooks" });

describe("App", () => {
  it("sidebar_lists_only_own_hooks", async () => {
    setup(fixture());
    const list = await myHooks();
    expect(within(list).getAllByRole("listitem")).toHaveLength(2);
    expect(within(list).getByText("Git status on session start")).toBeTruthy();
    expect(within(list).getByText("Finished working")).toBeTruthy();
    const other = screen.getByRole("button", { name: /Other hooks \(3\)/ });
    expect(other.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByText("echo one")).toBeNull();
    await fireEvent.click(other);
    expect(other.getAttribute("aria-expanded")).toBe("true");
    expect(await screen.findByText("echo one")).toBeTruthy();
    expect(screen.getByText("echo three")).toBeTruthy();
  });

  it("golden_report_renders", async () => {
    // Status "denied" so the pending detail shows Go's own `steps` text.
    const rep = structuredClone(golden) as unknown as Report;
    rep.pending[0].status = "denied";
    setup(rep);
    const list = await myHooks();
    for (const h of rep.hooks) expect(within(list).getByText(h.name)).toBeTruthy();
    const permStatuses = ["granted", "denied", "not-asked", "unknown"];
    for (const h of rep.hooks) {
      expect(Object.keys(HOOK_STATUS_TEXT)).toContain(h.status);
      for (const n of h.needs) expect(permStatuses).toContain(n.status);
    }
    for (const p of rep.pending) expect(permStatuses).toContain(p.status);
    const pending = screen.getByRole("list", { name: "Pending" });
    await fireEvent.click(within(pending).getByText(rep.pending[0].title));
    expect(typeof rep.pending[0].steps).toBe("string");
    expect(await screen.findByText(rep.pending[0].steps)).toBeTruthy();
  });

  it("pending_detail_not_askable_shows_steps_and_no_ask_button", async () => {
    const rep = fixture();
    rep.pending[0] = { ...rep.pending[0], askable: false, status: "not-asked" };
    setup(rep);
    await myHooks();
    const pending = screen.getByRole("list", { name: "Pending" });
    await fireEvent.click(within(pending).getByText(rep.pending[0].title));
    expect(await screen.findByText(rep.pending[0].steps)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Ask for permission" })).toBeNull();
    expect(screen.getByRole("button", { name: "Open Settings" })).toBeTruthy();
  });

  it("pending_detail_unknown_without_hint_falls_back_to_steps", async () => {
    const rep = fixture();
    rep.pending[0] = { ...rep.pending[0], status: "unknown", unknownHint: "" };
    setup(rep);
    await myHooks();
    const pending = screen.getByRole("list", { name: "Pending" });
    await fireEvent.click(within(pending).getByText(rep.pending[0].title));
    expect(await screen.findByText(rep.pending[0].steps)).toBeTruthy();
  });

  it("golden_questions_render_controls", async () => {
    // Every golden hook shows exactly the controls its questions describe, and the note only if it has one.
    const rep = structuredClone(golden) as unknown as Report;
    setup(rep);
    const list = await myHooks();
    for (const h of rep.hooks) {
      await fireEvent.click(within(list).getByText(h.name));
      for (const q of h.questions) {
        expect(await screen.findByLabelText(q.label)).toBeTruthy();
        expect(screen.getByLabelText(q.label).tagName).toBe(q.kind === "sound" ? "SELECT" : "INPUT");
      }
      const form = screen.getByRole("group", { name: "Options" });
      expect(form.querySelectorAll("input, select")).toHaveLength(h.questions.length);
      if (h.note) expect(screen.getByText(h.note)).toBeTruthy();
      else expect(document.querySelector(".note")).toBeNull();
    }
    expect(rep.hooks.some((h) => h.note)).toBe(true);
    expect(rep.hooks.some((h) => !h.note)).toBe(true);
  });

  it("add_menu_lists_available_only", async () => {
    setup(fixture());
    await myHooks();
    await fireEvent.click(screen.getByRole("button", { name: "+ Add hook" }));
    const menu = screen.getByRole("menu");
    const items = within(menu).getAllByRole("menuitem");
    expect(items).toHaveLength(1);
    expect(items[0].textContent).toContain("Input needed");
    expect(within(menu).queryByText("Finished working")).toBeNull();
  });

  it("add_menu_hidden_when_nothing_available", async () => {
    setup(fixture({ available: [] }));
    await myHooks();
    expect(screen.queryByRole("button", { name: "+ Add hook" })).toBeNull();
  });

  it("save_sends_answers", async () => {
    const calls = setup(fixture());
    const list = await myHooks();
    await fireEvent.click(within(list).getByText("Finished working"));
    const sound = (await screen.findByLabelText("Sound")) as HTMLSelectElement;
    await fireEvent.change(sound, { target: { value: "Ping" } });
    await fireEvent.click(screen.getByRole("button", { name: "Save & apply" }));
    await waitFor(() => expect(count(calls, "apply_hook")).toBe(1));
    expect(calls.find((c) => c.cmd === "apply_hook")?.args).toEqual({
      hookType: "finished",
      answers: { untrackedDirty: false, sound: "Ping", clickToFocus: true },
    });
  });

  it("add_form_uses_defaults_and_applies", async () => {
    const calls = setup(fixture());
    await myHooks();
    await fireEvent.click(screen.getByRole("button", { name: "+ Add hook" }));
    await fireEvent.click(screen.getByRole("menuitem", { name: /Input needed/ }));
    expect(((await screen.findByLabelText("Sound")) as HTMLSelectElement).value).toBe("Ping");
    await fireEvent.click(screen.getByRole("button", { name: "Save & apply" }));
    await waitFor(() => expect(count(calls, "apply_hook")).toBe(1));
    expect(calls.find((c) => c.cmd === "apply_hook")?.args).toEqual({
      hookType: "input-needed",
      answers: { untrackedDirty: false, sound: "Ping", clickToFocus: true },
    });
  });

  it("pending_open_settings", async () => {
    const rep = fixture();
    const calls = setup(rep);
    await myHooks();
    const pending = screen.getByRole("list", { name: "Pending" });
    await fireEvent.click(within(pending).getByText("Control Terminal"));
    expect(
      await screen.findByText("Open System Settings → Privacy & Security → Automation → Personalization Setup, and turn on Terminal."),
    ).toBeTruthy();
    expect(screen.getByText("Affects: Input needed")).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Open Settings" }));
    await waitFor(() => expect(count(calls, "open_settings")).toBe(1));
    expect(calls.find((c) => c.cmd === "open_settings")?.args).toEqual({ url: rep.pending[0].settingsUrl });
  });

  it("pending_unknown_says_terminal_may_not_be_running", async () => {
    const rep = fixture();
    rep.pending[0].status = "unknown";
    setup(rep);
    await myHooks();
    await fireEvent.click(within(screen.getByRole("list", { name: "Pending" })).getByText("Control Terminal"));
    expect(
      await screen.findByText("Couldn't check this yet. Terminal may not be running. Click Ask for permission to check again."),
    ).toBeTruthy();
    expect(screen.queryByText(/hasn't asked/)).toBeNull();
    expect(screen.getByRole("button", { name: "Ask for permission" })).toBeTruthy();
  });

  it("sidebar_explains_the_missing_dock_icon", async () => {
    setup(fixture());
    expect(await screen.findByText("No Dock icon. Open again from Spotlight. ⌘Q quits.")).toBeTruthy();
  });

  it("pending_not_asked_offers_the_prompt", async () => {
    const rep = fixture();
    const granted = fixture({ pending: [] });
    rep.pending[0].status = "not-asked";
    let asked = false;
    const calls = setup(rep, {
      request_permission: () => ((asked = true), { status: "granted" }),
      get_state: () => (asked ? granted : rep),
    });
    await myHooks();
    await fireEvent.click(within(screen.getByRole("list", { name: "Pending" })).getByText("Control Terminal"));
    expect(await screen.findByText("macOS hasn't asked about this yet. Click Ask for permission, then choose Allow.")).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Ask for permission" }));
    await waitFor(() => expect(count(calls, "request_permission")).toBe(1));
    expect(calls.find((c) => c.cmd === "request_permission")?.args).toEqual({ id: "automation-terminal" });
    await waitFor(() => expect(screen.queryByRole("list", { name: "Pending" })).toBeNull());
  });

  it("settings_error_disables_writes", async () => {
    const calls = setup(fixture({ settingsError: "expected value at line 3 column 1" }));
    expect(
      await screen.findByText(
        "Your ~/.claude/settings.json can't be read: expected value at line 3 column 1. Fix it, then come back. Nothing will be changed until then.",
      ),
    ).toBeTruthy();
    const list = await myHooks();
    await fireEvent.click(within(list).getByText("Finished working"));
    const save = screen.getByRole("button", { name: "Save & apply" }) as HTMLButtonElement;
    expect(save.disabled).toBe(true);
    expect((screen.getByRole("button", { name: "Turn off" }) as HTMLButtonElement).disabled).toBe(true);
    expect((screen.getByRole("button", { name: "Remove" }) as HTMLButtonElement).disabled).toBe(true);
    await fireEvent.click(save);
    expect(count(calls, "apply_hook")).toBe(0);
    expect(screen.getAllByRole("button", { name: "Open log" }).length).toBeGreaterThan(0);
  });

  it("settings_error_from_sidecar_is_not_doubled", async () => {
    // The real sidecar message already ends with its own advice; the banner must not repeat it.
    const msg =
      "/h/.claude/settings.json isn't valid JSON (line 1, column 14): invalid character ','. Fix it, then reopen. Nothing will be changed until then.";
    setup(fixture({ hooks: [], available: [], settingsError: msg }));
    expect(
      await screen.findByText(
        "Your ~/.claude/settings.json can't be read: /h/.claude/settings.json isn't valid JSON (line 1, column 14): invalid character ','. Fix it, then come back. Nothing will be changed until then.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText(/already added/)).toBeNull();
    expect(screen.queryByText("No hooks yet")).toBeNull();
  });

  it("shows_serve_down_banner", async () => {
    setup(fixture());
    await myHooks();
    await new Promise((r) => setTimeout(r, 0));
    await emit("serve-status", { state: "down", reason: "Another program is already listening" });
    expect(
      await screen.findByText(
        "Not listening for hook events: Another program is already listening. Alerts won't show until this clears.",
      ),
    ).toBeTruthy();
  });

  it("other_hooks_render_commands_as_text", async () => {
    const hostile = `<img src=x onerror="window.pwned=1">`;
    setup(fixture({ otherHooks: [{ event: "Stop", matcher: "", command: hostile }] }));
    await myHooks();
    await fireEvent.click(screen.getByRole("button", { name: /Other hooks \(1\)/ }));
    expect(await screen.findByText(hostile)).toBeTruthy();
    expect(document.querySelector("img")).toBeNull();
    expect((window as unknown as { pwned?: number }).pwned).toBeUndefined();
  });

  it("remove_confirms_inline", async () => {
    const calls = setup(fixture());
    const list = await myHooks();
    await fireEvent.click(within(list).getByText("Finished working"));
    await fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(screen.getByText("Remove Finished working? This deletes its saved answers.")).toBeTruthy();
    expect(count(calls, "remove_hook")).toBe(0);
    expect(screen.getByRole("button", { name: "Cancel" })).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Remove" }));
    await waitFor(() => expect(count(calls, "remove_hook")).toBe(1));
    expect(calls.find((c) => c.cmd === "remove_hook")?.args).toEqual({ hookType: "finished" });
    expect(await screen.findByText("Removed Finished working. Your settings.json was backed up first.")).toBeTruthy();
  });

  it("uninstall_confirms_inline_then_calls_backend", async () => {
    const calls = setup(fixture(), { uninstall_app: () => ({ removedHooks: 2, backup: "/b/settings.json.bak" }) });
    await myHooks();
    await fireEvent.click(screen.getByRole("button", { name: "Uninstall…" }));
    expect(screen.getByRole("heading", { name: "Uninstall Personalization Setup" })).toBeTruthy();
    expect(
      screen.getByText(
        "Removes this app's hooks from ~/.claude/settings.json, then the app, its saved answers and its logs. Your other hooks and settings are kept, and a backup of settings.json is saved next to it.",
      ),
    ).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Uninstall" }));
    expect(screen.getByText("Uninstall now? This can't be undone.")).toBeTruthy();
    expect(count(calls, "uninstall_app")).toBe(0);
    await fireEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByText("Uninstall now? This can't be undone.")).toBeNull();
    expect(screen.getByRole("button", { name: "Uninstall" })).toBeTruthy();
    await fireEvent.click(screen.getByRole("button", { name: "Uninstall" }));
    await fireEvent.click(screen.getByRole("button", { name: "Uninstall" }));
    await waitFor(() => expect(count(calls, "uninstall_app")).toBe(1));
  });

  it("uninstall_error_shows_banner", async () => {
    setup(fixture(), {
      uninstall_app: () => {
        throw "pcs uninstall timed out after 15s";
      },
    });
    await myHooks();
    await fireEvent.click(screen.getByRole("button", { name: "Uninstall…" }));
    await fireEvent.click(screen.getByRole("button", { name: "Uninstall" }));
    await fireEvent.click(screen.getByRole("button", { name: "Uninstall" }));
    expect(await screen.findByText("pcs uninstall timed out after 15s")).toBeTruthy();
  });

  it("uninstall_disabled_when_settings_invalid", async () => {
    const calls = setup(fixture({ settingsError: "expected value at line 3 column 1" }));
    await screen.findByText(/can't be read/);
    await fireEvent.click(screen.getByRole("button", { name: "Uninstall…" }));
    const btn = screen.getByRole("button", { name: "Uninstall" }) as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    await fireEvent.click(btn);
    expect(count(calls, "uninstall_app")).toBe(0);
  });

  it("refreshes_on_window_focus", async () => {
    const calls = setup(fixture());
    await myHooks();
    await waitFor(() => expect(count(calls, "get_state")).toBe(1));
    window.dispatchEvent(new Event("focus"));
    await waitFor(() => expect(count(calls, "get_state")).toBe(2));
  });

  it("focus_refresh_does_not_clobber_newer_write", async () => {
    const before = fixture();
    const after = fixture({ hooks: [gitStatus, { ...finished, enabled: false, status: "off" }] });
    let gets = 0;
    let releaseStale: (r: Report) => void = () => {};
    const calls = setup(before, {
      get_state: () => {
        gets += 1;
        if (gets === 1) return before;
        return new Promise<Report>((res) => (releaseStale = res));
      },
      disable_hook: () => after,
    });
    const list = await myHooks();
    await fireEvent.click(within(list).getByText("Finished working"));
    window.dispatchEvent(new Event("focus"));
    await waitFor(() => expect(count(calls, "get_state")).toBe(2));
    await fireEvent.click(screen.getByRole("button", { name: "Turn off" }));
    expect(await screen.findByRole("button", { name: "Turn on" })).toBeTruthy();
    releaseStale(before);
    await new Promise((r) => setTimeout(r, 10));
    expect(screen.getByRole("button", { name: "Turn on" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Turn off" })).toBeNull();
  });

  it("spaces_note_on_notification_hooks", async () => {
    const note = SPACES_NOTE;
    setup(fixture({ hooks: [gitStatus, finished, inputNeeded], available: [] }));
    const list = await myHooks();
    await fireEvent.click(within(list).getByText("Input needed"));
    expect(await screen.findByText(note)).toBeTruthy();
    await fireEvent.click(within(list).getByText("Git status on session start"));
    await waitFor(() => expect(screen.queryByText(note)).toBeNull());
    expect(screen.getByLabelText("Count untracked files as dirty")).toBeTruthy();
  });

  it("empty_state_invites_add", async () => {
    setup(fixture({ hooks: [], available: [{ type: "finished", name: "Finished working", description: "d", defaults: finished.answers, questions: notifyQs }] }));
    expect(await screen.findByText(/No hooks yet/)).toBeTruthy();
    expect(screen.getByRole("button", { name: "+ Add hook" })).toBeTruthy();
  });
});

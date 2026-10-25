import { describe, expect, it } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/svelte";
import { mockIPC } from "@tauri-apps/api/mocks";
import { emit } from "@tauri-apps/api/event";
import Alert from "./Alert.svelte";

type View = { kind: "panel" | "toast"; title: string; message: string; focus: boolean };

function setup(label: string, view: View) {
  window.history.replaceState(null, "", `/alert.html?label=${label}`);
  const calls: { cmd: string; args: unknown }[] = [];
  mockIPC(
    (cmd, args) => {
      calls.push({ cmd, args });
      if (cmd === "alert_get") return view;
      return null;
    },
    { shouldMockEvents: true },
  );
  render(Alert);
  return calls;
}

const count = (calls: { cmd: string }[], cmd: string) => calls.filter((c) => c.cmd === cmd).length;

const panel = (over: Partial<View> = {}): View => ({
  kind: "panel",
  title: "Claude Code — graphql-testing-framework",
  message: "Claude needs your permission to use Bash",
  focus: true,
  ...over,
});

describe("Alert", () => {
  it("alert_renders_message_as_text", async () => {
    const hostile = `<img src=x onerror="window.pwned=1">`;
    const calls = setup("panel-a", panel({ message: hostile }));
    expect(await screen.findByText(hostile)).toBeTruthy();
    expect(document.querySelector("img")).toBeNull();
    expect((window as unknown as { pwned?: number }).pwned).toBeUndefined();
    expect(calls.find((c) => c.cmd === "alert_get")?.args).toEqual({ label: "panel-a" });
  });

  it("toast_shows_finished_copy", async () => {
    setup("toast-1", { kind: "toast", title: "Claude Code — proj-x", message: "Finished working", focus: false });
    const project = await screen.findByText("proj-x");
    expect(project.classList.contains("project")).toBe(true);
    expect(screen.getByText("Claude Code")).toBeTruthy();
    const body = screen.getByText("Finished working");
    expect(body.classList.contains("body")).toBe(true);
    expect(screen.queryByRole("button", { name: "Dismiss" })).toBeNull();
  });

  it("panel_footer_reflects_focus", async () => {
    setup("panel-a", panel({ focus: true }));
    expect(await screen.findByText("Click to go to the session")).toBeTruthy();
  });

  it("panel_footer_reflects_focus (false)", async () => {
    setup("panel-b", panel({ focus: false }));
    expect(await screen.findByText("Click to dismiss")).toBeTruthy();
    expect(screen.queryByText("Click to go to the session")).toBeNull();
  });

  it("click_invokes_alert_click", async () => {
    const calls = setup("panel-a", panel());
    await fireEvent.click(await screen.findByText("graphql-testing-framework"));
    await waitFor(() => expect(count(calls, "alert_click")).toBe(1));
    expect(calls.find((c) => c.cmd === "alert_click")?.args).toEqual({ label: "panel-a" });
  });

  it("dismiss_does_not_click", async () => {
    const calls = setup("panel-a", panel());
    const x = await screen.findByRole("button", { name: "Dismiss" });
    await fireEvent.click(x);
    await waitFor(() => expect(count(calls, "alert_dismiss")).toBe(1));
    expect(count(calls, "alert_click")).toBe(0);
    expect(calls.find((c) => c.cmd === "alert_dismiss")?.args).toEqual({ label: "panel-a" });
  });

  it("updates_on_alert_updated_event", async () => {
    setup("panel-a", panel({ message: "first" }));
    await screen.findByText("first");
    // Let the listen() registration settle before emitting.
    await new Promise((r) => setTimeout(r, 0));
    await emit("alert-updated", panel({ message: "second", focus: false }));
    expect(await screen.findByText("second")).toBeTruthy();
    expect(screen.queryByText("first")).toBeNull();
    expect(screen.getByText("Click to dismiss")).toBeTruthy();
  });
});

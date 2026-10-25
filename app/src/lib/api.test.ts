import { describe, expect, it } from "vitest";
import { mockIPC } from "@tauri-apps/api/mocks";
import { emit } from "@tauri-apps/api/event";
import { api, ApiError, onServeStatus } from "./api";

describe("api", () => {
  it("wraps_errors", async () => {
    mockIPC(() => {
      throw "settings changed on disk, try again";
    });
    const err = await api.state().catch((e) => e);
    expect(err).toBeInstanceOf(ApiError);
    expect(err.message).toBe("settings changed on disk, try again");
  });

  it("sends_camelCase_args", async () => {
    let seen: unknown;
    mockIPC((cmd, args) => {
      seen = { cmd, args };
      return {};
    });
    await api.enable("finished");
    expect(seen).toEqual({ cmd: "enable_hook", args: { hookType: "finished" } });
  });

  it("serve_status_event", async () => {
    mockIPC(() => null, { shouldMockEvents: true });
    const got: unknown[] = [];
    const un = await onServeStatus((s) => got.push(s));
    await emit("serve-status", { state: "down", reason: "x" });
    expect(got).toEqual([{ state: "down", reason: "x" }]);
    un();
  });
});

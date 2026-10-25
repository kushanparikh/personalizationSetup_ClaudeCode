// Shared test setup: jsdom DOM + Tauri IPC mocks reset between tests.
import { afterEach } from "vitest";
import { clearMocks } from "@tauri-apps/api/mocks";
import { cleanup } from "@testing-library/svelte";

// jsdom has no WebCrypto getRandomValues on older versions; the Tauri mocks need it.
if (!window.crypto?.getRandomValues) {
  Object.defineProperty(window, "crypto", {
    value: { getRandomValues: (a: Uint32Array) => a.map(() => Math.floor(Math.random() * 2 ** 32)) },
  });
}

afterEach(() => {
  // Unmount first so components can unlisten while the Tauri mocks still exist.
  cleanup();
  clearMocks();
  window.history.replaceState(null, "", "/");
});

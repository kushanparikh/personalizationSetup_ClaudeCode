<script lang="ts">
  import { onMount } from "svelte";
  import { api, onServeStatus } from "./lib/api";
  import { AppStore, type Selection } from "./lib/store.svelte";
  import Sidebar from "./components/Sidebar.svelte";
  import Banner from "./components/Banner.svelte";
  import HookDetail from "./components/HookDetail.svelte";
  import AddHook from "./components/AddHook.svelte";
  import PendingDetail from "./components/PendingDetail.svelte";
  import OtherHooks from "./components/OtherHooks.svelte";

  const store = new AppStore();

  // What the content area actually shows: the stored selection when it still exists, else the first hook.
  const current = $derived.by((): Selection => {
    const r = store.report;
    const s = store.selection;
    if (!r) return { kind: "none" };
    if (s.kind === "hook" && r.hooks.some((h) => h.type === s.type)) return s;
    if (s.kind === "add" && (s.type === null || r.available.some((d) => d.type === s.type))) return s;
    if (s.kind === "pending" && r.pending.some((p) => p.id === s.id)) return s;
    if (s.kind === "other" && r.otherHooks.length > 0) return s;
    if (r.hooks.length > 0) return { kind: "hook", type: r.hooks[0].type };
    return { kind: "none" };
  });

  const hook = $derived(current.kind === "hook" ? store.report?.hooks.find((h) => h.type === current.type) : undefined);
  const def = $derived(
    current.kind === "add" && current.type ? store.report?.available.find((d) => d.type === current.type) : undefined,
  );
  const item = $derived(current.kind === "pending" ? store.report?.pending.find((p) => p.id === current.id) : undefined);

  // The sidecar's message may already end with its own "Fix it, …" advice and a full stop; the banner adds its own.
  const settingsProblem = $derived(
    (store.report?.settingsError ?? "")
      .replace(/\s*Fix it\b.*$/s, "")
      .trim()
      .replace(/\.+$/, ""),
  );

  function openLog() {
    api.openLog().catch((e) => (store.error = e instanceof Error ? e.message : String(e)));
  }

  // Coming back to the window re-reads settings and permissions (spec §4.3) without clearing messages.
  function onFocus() {
    if (store.busy) return;
    api
      .state()
      .then((r) => (store.report = r))
      .catch((e) => (store.error = e instanceof Error ? e.message : String(e)));
  }

  onMount(() => {
    store.refresh();
    let unlisten: (() => void) | undefined;
    let gone = false;
    let heard = false;
    onServeStatus((s) => {
      heard = true;
      store.serve = s;
    }).then((u) => (gone ? u() : (unlisten = u)));
    api
      .serveStatus()
      .then((s) => {
        if (!heard) store.serve = s;
      })
      .catch(() => {});
    return () => {
      gone = true;
      unlisten?.();
    };
  });
</script>

<svelte:window onfocus={onFocus} />

<div class="shell">
  {#if store.report}
    <Sidebar {store} report={store.report} {current} />
  {:else}
    <div class="sidebar-placeholder"></div>
  {/if}

  <main class="content">
    <div class="banners">
      {#if store.report?.settingsError}
        <Banner
          tone="danger"
          text="Your ~/.claude/settings.json can't be read: {settingsProblem}. Fix it, then come back. Nothing will be changed until then."
          onOpenLog={openLog}
        />
      {/if}
      {#if store.serve.state === "down"}
        <Banner
          tone="wait"
          text="Not listening for hook events: {store.serve.reason}. Alerts won't show until this clears."
          onOpenLog={openLog}
        />
      {/if}
      {#if store.error}
        <Banner tone="danger" text={store.error} onOpenLog={openLog} onDismiss={() => (store.error = null)} />
      {/if}
      {#if store.notice}
        <Banner tone="done" text={store.notice} onOpenLog={openLog} onDismiss={() => (store.notice = null)} />
      {/if}
    </div>

    <div class="pane">
      {#if !store.report}
        <p class="loading">{store.busy ? "Reading your settings…" : ""}</p>
      {:else if hook}
        {#key `${hook.type}|${hook.enabled}|${JSON.stringify(hook.answers)}`}
          <HookDetail {store} report={store.report} {hook} />
        {/key}
      {:else if def}
        {#key def.type}
          <AddHook {store} {def} />
        {/key}
      {:else if item}
        <PendingDetail {store} {item} />
      {:else if current.kind === "other"}
        <OtherHooks hooks={store.report.otherHooks} />
      {:else if store.report.settingsError}
        <p class="loading">Your hooks show here once settings.json can be read.</p>
      {:else}
        <section class="empty">
          <h1>{store.report.hooks.length === 0 ? "No hooks yet" : "Add a hook"}</h1>
          {#if store.report.available.length > 0}
            <p>Pick what Claude Code should do. You can add more any time from <span class="nowrap">+ Add hook</span>.</p>
            <ul>
              {#each store.report.available as d (d.type)}
                <li>
                  <button type="button" class="choice" onclick={() => (store.selection = { kind: "add", type: d.type })}>
                    <span class="choice-name">{d.name}</span>
                    <span class="choice-desc">{d.description}</span>
                  </button>
                </li>
              {/each}
            </ul>
          {:else}
            <p>Every hook type is already added.</p>
          {/if}
        </section>
      {/if}
    </div>
  </main>
</div>

<style>
  :global(html),
  :global(body),
  :global(#app) {
    margin: 0;
    height: 100%;
  }
  :global(body) {
    background: var(--c-surface);
    color: var(--c-ink);
    font-family: var(--f-ui);
    font-size: var(--t-sm);
    -webkit-font-smoothing: antialiased;
    -webkit-user-select: none;
    user-select: none;
    cursor: default;
  }

  /* Buttons shared by every detail view. */
  :global(.btn) {
    font: inherit;
    font-size: var(--t-sm);
    font-weight: var(--w-medium);
    line-height: 1;
    padding: 7px 13px;
    border-radius: var(--r-md);
    border: 1px solid var(--c-hairline);
    background: var(--c-surface-raised);
    color: var(--c-ink);
    cursor: default;
  }
  :global(.btn:hover:not(:disabled)) {
    background: var(--c-hover);
  }
  :global(.btn.primary) {
    background: var(--c-ink);
    border-color: var(--c-ink);
    color: var(--c-on-ink);
  }
  :global(.btn.primary:hover:not(:disabled)) {
    background: color-mix(in srgb, var(--c-ink) 85%, var(--c-surface));
  }
  :global(.btn.danger) {
    background: var(--c-danger);
    border-color: var(--c-danger);
    color: var(--c-on-ink);
  }
  :global(.btn.danger:hover:not(:disabled)) {
    background: color-mix(in srgb, var(--c-danger) 85%, var(--c-ink));
  }
  :global(.btn.quiet-danger) {
    border-color: transparent;
    background: transparent;
    color: var(--c-danger);
  }
  :global(.btn:disabled) {
    opacity: 0.45;
  }
  :global(.btn:focus-visible) {
    outline: 2px solid var(--c-focus);
    outline-offset: 2px;
  }
  :global(.link) {
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    color: var(--c-focus);
    cursor: pointer;
  }
  :global(.link:hover) {
    text-decoration: underline;
  }
  :global(.link:focus-visible) {
    outline: 2px solid var(--c-focus);
    outline-offset: 2px;
    border-radius: var(--r-sm);
  }

  .shell {
    display: grid;
    grid-template-columns: clamp(190px, 26vw, 236px) minmax(0, 1fr);
    height: 100vh;
  }
  .sidebar-placeholder {
    background: var(--c-sidebar);
    border-right: 1px solid var(--c-hairline);
  }
  .content {
    display: flex;
    flex-direction: column;
    min-width: 0;
    height: 100vh;
    overflow: hidden;
  }
  .banners {
    flex: none;
  }
  .pane {
    flex: 1;
    overflow-y: auto;
    padding: 28px 36px 40px;
  }
  .pane > :global(*) {
    max-width: 600px;
  }
  .loading {
    margin: 0;
    color: var(--c-muted);
  }

  .empty h1 {
    margin: 0;
    font-family: var(--f-name);
    font-size: var(--t-xl);
    font-weight: var(--w-semibold);
    letter-spacing: -0.015em;
  }
  .empty p {
    margin: 10px 0 0;
    max-width: 52ch;
    font-size: var(--t-md);
    line-height: 1.4;
    color: var(--c-muted);
  }
  .nowrap {
    white-space: nowrap;
  }
  .empty ul {
    list-style: none;
    margin: 22px 0 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .choice {
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 3px;
    width: 100%;
    padding: 12px 14px 12px calc(var(--lamp-w) + 16px);
    border: 1px solid var(--c-hairline);
    border-radius: var(--r-md);
    background: var(--c-surface-raised);
    color: var(--c-ink);
    font: inherit;
    text-align: left;
    cursor: default;
    overflow: hidden;
  }
  .choice::before {
    content: "";
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: var(--lamp-w);
    background: var(--c-hairline);
    transition: background 140ms var(--ease-out);
  }
  .choice:hover::before,
  .choice:focus-visible::before {
    background: var(--c-lamp-done);
  }
  .choice:focus-visible {
    outline: 2px solid var(--c-focus);
    outline-offset: 2px;
  }
  .choice-name {
    font-family: var(--f-name);
    font-size: var(--t-md);
    font-weight: var(--w-semibold);
  }
  .choice-desc {
    font-size: var(--t-sm);
    color: var(--c-muted);
    line-height: 1.35;
  }
  @media (max-width: 760px) {
    .pane {
      padding: 22px 24px 32px;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .choice::before {
      transition: none;
    }
  }
</style>

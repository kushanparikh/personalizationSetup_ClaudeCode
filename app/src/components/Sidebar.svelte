<script lang="ts">
  import type { AppStore, Selection } from "../lib/store.svelte";
  import type { HookStatus, Report } from "../lib/types";

  let { store, report, current }: { store: AppStore; report: Report; current: Selection } = $props();

  let menuOpen = $state(false);
  let addButton = $state<HTMLButtonElement>();
  let menuEl = $state<HTMLDivElement>();

  const glyph: Record<HookStatus, string> = { ok: "✅", pending: "⚠️", off: "⏸" };
  const statusText: Record<HookStatus, string> = {
    ok: "Working",
    pending: "Waiting on permission",
    off: "Off",
  };

  const otherOpen = $derived(current.kind === "other");

  function select(s: Selection) {
    menuOpen = false;
    store.selection = s;
  }

  function toggleMenu() {
    menuOpen = !menuOpen;
    if (menuOpen) queueMicrotask(() => menuEl?.querySelector<HTMLButtonElement>("[role=menuitem]")?.focus());
  }

  function menuKey(e: KeyboardEvent) {
    const items = [...(menuEl?.querySelectorAll<HTMLButtonElement>("[role=menuitem]") ?? [])];
    const i = items.indexOf(document.activeElement as HTMLButtonElement);
    if (e.key === "Escape") {
      e.preventDefault();
      menuOpen = false;
      addButton?.focus();
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      items[(i + 1) % items.length]?.focus();
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      items[(i - 1 + items.length) % items.length]?.focus();
    }
  }

  function outside(e: MouseEvent) {
    if (!menuOpen) return;
    const t = e.target as Node;
    if (!menuEl?.contains(t) && !addButton?.contains(t)) menuOpen = false;
  }

  const isHook = (t: string) => current.kind === "hook" && current.type === t;
  const isPending = (id: string) => current.kind === "pending" && current.id === id;
</script>

<svelte:window onclick={outside} />

<nav class="sidebar" aria-label="Hooks">
  <section>
    <h2 id="sb-mine">My hooks</h2>
    {#if report.hooks.length > 0}
      <ul aria-labelledby="sb-mine">
        {#each report.hooks as h (h.type)}
          <li>
            <button
              type="button"
              class="row"
              class:selected={isHook(h.type)}
              aria-current={isHook(h.type) ? "page" : undefined}
              title={statusText[h.status]}
              onclick={() => select({ kind: "hook", type: h.type })}
            >
              <span class="glyph" aria-hidden="true">{glyph[h.status]}</span>
              <span class="name">{h.name}</span>
              <span class="sr">{statusText[h.status]}</span>
            </button>
          </li>
        {/each}
      </ul>
    {:else}
      <ul aria-labelledby="sb-mine"></ul>
      <p class="none">None yet</p>
    {/if}

    {#if report.available.length > 0}
      <div class="add">
        <button
          type="button"
          class="row add-btn"
          class:selected={current.kind === "add"}
          aria-haspopup="menu"
          aria-expanded={menuOpen}
          bind:this={addButton}
          onclick={toggleMenu}>+ Add hook</button
        >
        {#if menuOpen}
          <!-- svelte-ignore a11y_interactive_supports_focus -->
          <div class="menu" role="menu" aria-label="Add hook" tabindex="-1" bind:this={menuEl} onkeydown={menuKey}>
            {#each report.available as d (d.type)}
              <button type="button" role="menuitem" class="item" onclick={() => select({ kind: "add", type: d.type })}>
                <span class="item-name">{d.name}</span>
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
  </section>

  {#if report.otherHooks.length > 0}
    <section>
      <button
        type="button"
        class="row disclosure"
        class:selected={otherOpen}
        aria-expanded={otherOpen}
        onclick={() => select(otherOpen ? { kind: "none" } : { kind: "other" })}
      >
        <svg class="chev" class:open={otherOpen} viewBox="0 0 8 8" width="8" height="8" aria-hidden="true">
          <path d="M2.5 1.2L5.6 4 2.5 6.8" fill="none" stroke="currentColor" stroke-width="1.3" stroke-linecap="round" stroke-linejoin="round" />
        </svg>
        <span class="name">Other hooks ({report.otherHooks.length})</span>
      </button>
      {#if otherOpen}
        <ul class="others" aria-label="Other hooks">
          {#each report.otherHooks as o, i (i)}
            <li>
              <span>{o.event}</span>{#if o.matcher}<span class="matcher">{o.matcher}</span>{/if}
            </li>
          {/each}
        </ul>
      {/if}
    </section>
  {/if}

  {#if report.pending.length > 0}
    <section>
      <h2 id="sb-pending">Pending ({report.pending.length})</h2>
      <ul aria-label="Pending">
        {#each report.pending as p (p.id)}
          <li>
            <button
              type="button"
              class="row"
              class:selected={isPending(p.id)}
              aria-current={isPending(p.id) ? "page" : undefined}
              onclick={() => select({ kind: "pending", id: p.id })}
            >
              <span class="pip" aria-hidden="true"></span>
              <span class="name">{p.title}</span>
            </button>
          </li>
        {/each}
      </ul>
    </section>
  {/if}

  <p class="dockless">No Dock icon. Open again from Spotlight. ⌘Q quits.</p>
</nav>

<style>
  .sidebar {
    box-sizing: border-box;
    height: 100%;
    overflow-y: auto;
    padding: 14px 10px 20px;
    background: var(--c-sidebar);
    border-right: 1px solid var(--c-hairline);
    display: flex;
    flex-direction: column;
    gap: 18px;
  }
  section {
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  h2 {
    margin: 0 8px 4px;
    font-size: var(--t-xs);
    font-weight: var(--w-semibold);
    color: var(--c-muted);
    line-height: var(--lh-tight);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .dockless {
    margin: auto 8px 0;
    font-size: var(--t-xs);
    color: var(--c-muted);
    line-height: 1.35;
  }
  .none {
    margin: 0 8px 2px;
    font-size: var(--t-sm);
    color: var(--c-muted);
  }

  .row {
    box-sizing: border-box;
    display: flex;
    align-items: center;
    gap: 8px;
    width: 100%;
    min-height: 28px;
    padding: 4px 8px;
    border: 0;
    border-radius: var(--r-md);
    background: transparent;
    color: var(--c-ink);
    font: inherit;
    font-size: var(--t-sm);
    text-align: left;
    cursor: default;
  }
  .row:hover {
    background: var(--c-hover);
  }
  .row.selected {
    background: var(--c-selected);
  }
  .row:focus-visible {
    outline: 2px solid var(--c-focus);
    outline-offset: -2px;
  }
  .glyph {
    flex: none;
    width: 18px;
    text-align: center;
    font-size: 13px;
  }
  .name {
    flex: 1;
    min-width: 0;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .pip {
    flex: none;
    width: 18px;
    display: grid;
    place-items: center;
  }
  .pip::before {
    content: "";
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--c-lamp-wait);
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }

  .add {
    position: relative;
    margin-top: 2px;
  }
  .add-btn {
    color: var(--c-muted);
  }
  .add-btn:hover,
  .add-btn[aria-expanded="true"] {
    color: var(--c-ink);
  }
  .menu {
    position: absolute;
    z-index: 5;
    top: calc(100% + 4px);
    left: 0;
    right: 0;
    padding: 4px;
    background: var(--c-surface-raised);
    border: 1px solid var(--c-hairline);
    border-radius: var(--r-lg);
    box-shadow: 0 10px 28px -8px color-mix(in srgb, var(--c-ink) 35%, transparent);
    outline: none;
  }
  .item {
    display: flex;
    flex-direction: column;
    gap: 2px;
    width: 100%;
    padding: 7px 10px;
    border: 0;
    border-radius: var(--r-md);
    background: transparent;
    color: var(--c-ink);
    font: inherit;
    text-align: left;
    cursor: default;
    outline: none;
  }
  .item:hover,
  .item:focus-visible {
    background: var(--c-hover);
  }
  .item-name {
    font-family: var(--f-name);
    font-size: var(--t-sm);
    font-weight: var(--w-semibold);
  }

  .disclosure {
    color: var(--c-muted);
  }
  .disclosure.selected {
    color: var(--c-ink);
  }
  .chev {
    flex: none;
    width: 18px;
    transition: transform 160ms var(--ease-out);
  }
  .chev.open {
    transform: rotate(90deg);
  }
  .others {
    padding: 2px 0 0 34px;
    gap: 3px;
  }
  .others li {
    font-size: var(--t-xs);
    color: var(--c-muted);
    display: flex;
    gap: 6px;
    min-width: 0;
  }
  .matcher {
    font-family: var(--f-mono);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  @media (prefers-reduced-motion: reduce) {
    .chev {
      transition: none;
    }
  }
</style>

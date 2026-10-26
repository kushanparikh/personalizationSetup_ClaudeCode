<script lang="ts">
  import type { AppStore } from "../lib/store.svelte";
  import type { PendingItem } from "../lib/types";

  let { store, item }: { store: AppStore; item: PendingItem } = $props();
  // macOS lists an app under Automation only after it has asked once, so Settings can't help yet.
  let canAsk = $derived(item.askable && (item.status === "not-asked" || item.status === "unknown"));
</script>

<article class="detail">
  <header>
    <span class="lamp" aria-hidden="true"></span>
    <div>
      <h1>{item.title}</h1>
      <p class="status">{item.status === "denied" ? "Not allowed" : "Not allowed yet"}</p>
    </div>
  </header>

  {#if item.status === "not-asked" && item.askable}
    <p class="steps">macOS hasn't asked about this yet. Click Ask for permission, then choose Allow.</p>
  {:else if item.status === "unknown" && item.unknownHint}
    <p class="steps">{item.unknownHint}</p>
  {:else if item.steps}
    <p class="steps">{item.steps}</p>
  {/if}

  <p class="affects">Affects: {item.affects.join(", ")}</p>

  <div class="actions">
    {#if canAsk}
      <button type="button" class="btn primary" disabled={store.busy} onclick={() => store.askPermission(item.id)}>Ask for permission</button>
      <button type="button" class="btn" onclick={() => store.openSettings(item.settingsUrl)}>Open Settings</button>
    {:else}
      <button type="button" class="btn primary" onclick={() => store.openSettings(item.settingsUrl)}>Open Settings</button>
    {/if}
  </div>
  <p class="hint">Come back to this window when you're done. It checks again automatically.</p>
</article>

<style>
  header {
    display: flex;
    align-items: stretch;
    gap: 14px;
  }
  .lamp {
    flex: none;
    width: var(--lamp-w);
    border-radius: 2px;
    background: var(--c-lamp-wait);
  }
  h1 {
    margin: 0;
    font-family: var(--f-name);
    font-size: var(--t-xl);
    font-weight: var(--w-semibold);
    letter-spacing: -0.015em;
    line-height: var(--lh-tight);
  }
  .status {
    margin: 3px 0 0;
    font-size: var(--t-sm);
    color: var(--c-muted);
  }
  .steps {
    margin: 22px 0 0;
    padding: 18px 0 0;
    border-top: 1px solid var(--c-hairline);
    max-width: 60ch;
    font-size: var(--t-md);
    line-height: 1.4;
  }
  .affects {
    margin: 18px 0 0;
    font-size: var(--t-sm);
    color: var(--c-muted);
  }
  .actions {
    margin-top: 22px;
    display: flex;
    gap: 8px;
  }
  .hint {
    margin: 10px 0 0;
    font-size: var(--t-xs);
    color: var(--c-muted);
  }
</style>

<script lang="ts">
  import type { AppStore } from "../lib/store.svelte";

  let { store }: { store: AppStore } = $props();

  let confirming = $state(false);

  const canWrite = $derived(store.canWrite && !store.uninstalled);
</script>

<article class="detail">
  <h1>Uninstall Personalization Setup</h1>
  <p class="desc">
    Removes this app's hooks from ~/.claude/settings.json, then the app, its saved answers and its logs. Your other hooks
    and settings are kept, and a backup of settings.json is saved next to it.
  </p>

  <div class="actions">
    {#if store.busy && confirming}
      <p class="working" role="status">Uninstalling…</p>
    {:else if confirming}
      <div class="confirm" role="group" aria-label="Confirm uninstall">
        <p>Uninstall now? This can't be undone.</p>
        <button
          type="button"
          class="btn danger"
          disabled={!canWrite}
          onclick={async () => {
            await store.uninstall();
            if (!store.uninstalled) confirming = false;
          }}>Uninstall</button
        >
        <button type="button" class="btn" onclick={() => (confirming = false)}>Cancel</button>
      </div>
    {:else}
      <button type="button" class="btn quiet-danger" disabled={!canWrite} onclick={() => (confirming = true)}
        >Uninstall</button
      >
    {/if}
  </div>
</article>

<style>
  h1 {
    margin: 0;
    font-family: var(--f-name);
    font-size: var(--t-xl);
    font-weight: var(--w-semibold);
    letter-spacing: -0.015em;
    line-height: var(--lh-tight);
  }
  .desc {
    margin: 14px 0 0;
    max-width: 60ch;
    font-size: var(--t-md);
    line-height: 1.4;
  }
  .actions {
    margin-top: 26px;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
  .working {
    margin: 0;
    font-size: var(--t-sm);
    color: var(--c-muted);
  }
  .confirm {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
    width: 100%;
    padding: 10px 12px;
    border-radius: var(--r-md);
    background: var(--c-wash-danger);
  }
  .confirm p {
    margin: 0 8px 0 0;
    flex: 1 1 240px;
    font-size: var(--t-sm);
  }
</style>

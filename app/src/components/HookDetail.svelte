<script lang="ts">
  import HookForm from "./HookForm.svelte";
  import type { AppStore } from "../lib/store.svelte";
  import { HOOK_STATUS_TEXT, type Answers, type HookView, type PermId, type PermStatus, type Report } from "../lib/types";

  let { store, report, hook }: { store: AppStore; report: Report; hook: HookView } = $props();

  // The parent re-creates this component when the saved answers change, so a plain copy is right.
  // svelte-ignore state_referenced_locally
  let answers = $state<Answers>({ ...hook.answers });
  let confirming = $state(false);

  const permText: Record<PermStatus, string> = {
    granted: "Allowed",
    denied: "Not allowed",
    "not-asked": "Not asked yet",
    unknown: "Not checked yet",
  };

  const canWrite = $derived(store.canWrite);
  const pendingFor = (id: PermId) => report.pending.some((p) => p.id === id);
</script>

<article class="detail" data-status={hook.status}>
  <header>
    <span class="lamp" aria-hidden="true"></span>
    <div>
      <h1>{hook.name}</h1>
      <p class="status">{HOOK_STATUS_TEXT[hook.status]}</p>
    </div>
  </header>
  {#if hook.description}<p class="desc">{hook.description}</p>{/if}

  <div class="group">
    <HookForm questions={hook.questions} note={hook.note} bind:answers disabled={!canWrite} />
  </div>

  {#if hook.needs.length > 0}
    <div class="group">
      <h2>Needs</h2>
      <ul class="needs">
        {#each hook.needs as n (n.id)}
          <li>
            <span class="perm">{n.title}</span>
            <span class="perm-status" class:granted={n.status === "granted"}>{permText[n.status]}</span>
            {#if n.status !== "granted" && pendingFor(n.id)}
              <button type="button" class="link" onclick={() => (store.selection = { kind: "pending", id: n.id })}
                >How to allow</button
              >
            {/if}
          </li>
        {/each}
      </ul>
    </div>
  {/if}

  <div class="actions">
    {#if confirming}
      <div class="confirm" role="group" aria-label="Confirm remove">
        <p>Remove {hook.name}? This deletes its saved answers.</p>
        <button
          type="button"
          class="btn danger"
          disabled={!canWrite}
          onclick={async () => {
            await store.remove(hook.type);
            confirming = false;
          }}>Remove</button
        >
        <button type="button" class="btn" onclick={() => (confirming = false)}>Cancel</button>
      </div>
    {:else}
      <button
        type="button"
        class="btn primary"
        disabled={!canWrite}
        onclick={() => store.saveAndApply(hook.type, { ...answers })}>Save & apply</button
      >
      <button type="button" class="btn" disabled={store.busy} onclick={() => store.sendTest(hook.type)}>Send test</button>
      <button type="button" class="btn" disabled={!canWrite} onclick={() => store.setEnabled(hook.type, !hook.enabled)}
        >{hook.enabled ? "Turn off" : "Turn on"}</button
      >
      <span class="spacer"></span>
      <button type="button" class="btn quiet-danger" disabled={!canWrite} onclick={() => (confirming = true)}>Remove</button>
    {/if}
  </div>
</article>

<style>
  .detail {
    --lamp: var(--c-lamp-done);
  }
  .detail[data-status="pending"] {
    --lamp: var(--c-lamp-wait);
  }
  .detail[data-status="off"] {
    --lamp: var(--c-hairline);
  }
  header {
    display: flex;
    align-items: stretch;
    gap: 14px;
  }
  .lamp {
    flex: none;
    width: var(--lamp-w);
    border-radius: 2px;
    background: var(--lamp);
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
  .desc {
    margin: 14px 0 0;
    max-width: 60ch;
    font-size: var(--t-md);
    line-height: 1.4;
  }
  .group {
    margin-top: 22px;
    padding-top: 18px;
    border-top: 1px solid var(--c-hairline);
  }
  h2 {
    margin: 0 0 10px;
    font-size: var(--t-sm);
    font-weight: var(--w-semibold);
  }
  .needs {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .needs li {
    display: flex;
    flex-wrap: wrap;
    align-items: baseline;
    gap: 10px;
    font-size: var(--t-sm);
  }
  .perm-status {
    color: var(--c-lamp-wait);
    font-weight: var(--w-medium);
  }
  .perm-status.granted {
    color: var(--c-lamp-done);
  }
  .actions {
    margin-top: 26px;
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px;
  }
  .spacer {
    flex: 1;
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

<script lang="ts">
  import HookForm from "./HookForm.svelte";
  import type { AppStore } from "../lib/store.svelte";
  import type { Answers, HookDef } from "../lib/types";

  let { store, def }: { store: AppStore; def: HookDef } = $props();

  // The parent keys this component by type, so copying the defaults once is right.
  // svelte-ignore state_referenced_locally
  let answers = $state<Answers>({ ...def.defaults });
</script>

<article class="detail">
  <header>
    <span class="lamp" aria-hidden="true"></span>
    <div>
      <h1>{def.name}</h1>
      <p class="status">Not added yet</p>
    </div>
  </header>
  {#if def.description}<p class="desc">{def.description}</p>{/if}

  <div class="group">
    <HookForm questions={def.questions} note={def.note} bind:answers disabled={!store.canWrite} />
  </div>

  <div class="actions">
    <button
      type="button"
      class="btn primary"
      disabled={!store.canWrite}
      onclick={() => store.saveAndApply(def.type, { ...answers })}>Save & apply</button
    >
    <button type="button" class="btn" onclick={() => (store.selection = { kind: "none" })}>Cancel</button>
  </div>
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
    background: repeating-linear-gradient(
      to bottom,
      var(--c-muted) 0 3px,
      transparent 3px 6px
    );
    opacity: 0.6;
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
  .actions {
    margin-top: 26px;
    display: flex;
    flex-wrap: wrap;
    gap: 8px;
  }
</style>

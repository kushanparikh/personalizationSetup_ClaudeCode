<script lang="ts">
  import { SOUNDS, type Answers, type HookType } from "../lib/types";

  // The questions for one hook type (spec §5). The parent owns `answers`.
  let { type, answers = $bindable(), disabled = false }: { type: HookType; answers: Answers; disabled?: boolean } =
    $props();

  const uid = $props.id();
  const notifies = $derived(type === "finished" || type === "input-needed");
</script>

<fieldset class="form" {disabled}>
  <legend class="sr">Options</legend>
  {#if type === "git-status"}
    <label class="check">
      <input type="checkbox" bind:checked={answers.untrackedDirty} />
      <span>Count untracked files as dirty</span>
    </label>
  {:else}
    <div class="field">
      <label for="{uid}-sound">Sound</label>
      <select id="{uid}-sound" bind:value={answers.sound}>
        {#each SOUNDS as s (s)}
          <option value={s}>{s === "none" ? "None" : s}</option>
        {/each}
      </select>
    </div>
    <label class="check">
      <input type="checkbox" bind:checked={answers.clickToFocus} />
      <span>Click focuses the session's terminal tab</span>
    </label>
  {/if}
</fieldset>

{#if notifies}
  <p class="note">
    If that session's window is on another desktop or full-screen Space, macOS won't switch to it. The tab is selected and
    Terminal comes forward.
  </p>
{/if}

<style>
  .form {
    margin: 0;
    padding: 0;
    border: 0;
    display: flex;
    flex-direction: column;
    gap: 12px;
    min-width: 0;
  }
  .sr {
    position: absolute;
    width: 1px;
    height: 1px;
    overflow: hidden;
    clip-path: inset(50%);
  }
  .field {
    display: grid;
    grid-template-columns: 64px minmax(0, 200px);
    align-items: center;
    gap: 10px;
    font-size: var(--t-sm);
  }
  .check {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: var(--t-sm);
    width: fit-content;
  }
  input[type="checkbox"] {
    margin: 0;
    width: 15px;
    height: 15px;
    accent-color: var(--c-focus);
  }
  select {
    font: inherit;
    font-size: var(--t-sm);
    padding: 3px 6px;
    border: 1px solid var(--c-hairline);
    border-radius: var(--r-sm);
    background: var(--c-surface-raised);
    color: var(--c-ink);
  }
  select:focus-visible,
  input:focus-visible {
    outline: 2px solid var(--c-focus);
    outline-offset: 1px;
  }
  fieldset:disabled {
    opacity: 0.55;
  }
  .note {
    margin: 12px 0 0;
    max-width: 60ch;
    font-size: var(--t-xs);
    line-height: 1.4;
    color: var(--c-muted);
  }
</style>

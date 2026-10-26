<script lang="ts">
  import { SOUNDS, type Answers, type Question } from "../lib/types";

  // The questions pcs describes for one hook type (spec §5). The parent owns `answers`.
  let {
    questions,
    note = "",
    answers = $bindable(),
    disabled = false,
  }: { questions: Question[]; note?: string; answers: Answers; disabled?: boolean } = $props();

  const uid = $props.id();
</script>

<fieldset class="form" {disabled}>
  <legend class="sr">Options</legend>
  {#each questions as q (q.key)}
    {#if q.kind === "bool"}
      <label class="check">
        <input type="checkbox" bind:checked={answers[q.key] as boolean} />
        <span>{q.label}</span>
      </label>
    {:else if q.kind === "sound"}
      <div class="field">
        <label for="{uid}-{q.key}">{q.label}</label>
        <select id="{uid}-{q.key}" bind:value={answers[q.key] as string}>
          {#each SOUNDS as s (s)}
            <option value={s}>{s === "none" ? "None" : s}</option>
          {/each}
        </select>
      </div>
    {/if}
  {/each}
</fieldset>

{#if note}
  <p class="note">{note}</p>
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

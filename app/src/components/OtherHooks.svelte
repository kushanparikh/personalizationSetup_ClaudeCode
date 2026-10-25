<script lang="ts">
  import type { OtherHook } from "../lib/types";

  // Read-only: hooks in settings.json that this app didn't add (spec §7). Commands are plain text.
  let { hooks }: { hooks: OtherHook[] } = $props();
</script>

<article class="detail">
  <h1>Other hooks</h1>
  <p class="desc">These are in your settings.json but weren't added by this app. They're shown here so you know they run; this app won't change them.</p>

  <ul class="list">
    {#each hooks as h, i (i)}
      <li>
        <div class="meta">
          <span class="event">{h.event}</span>
          {#if h.matcher}<span class="matcher">matches {h.matcher}</span>{/if}
        </div>
        <code class="cmd">{h.command}</code>
      </li>
    {/each}
  </ul>
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
    margin: 10px 0 0;
    max-width: 60ch;
    font-size: var(--t-sm);
    line-height: 1.4;
    color: var(--c-muted);
  }
  .list {
    list-style: none;
    margin: 20px 0 0;
    padding: 0;
    border-top: 1px solid var(--c-hairline);
  }
  .list li {
    padding: 12px 0;
    border-bottom: 1px solid var(--c-hairline);
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .meta {
    display: flex;
    align-items: baseline;
    gap: 10px;
    font-size: var(--t-sm);
  }
  .event {
    font-weight: var(--w-semibold);
  }
  .matcher {
    color: var(--c-muted);
  }
  .cmd {
    display: block;
    padding: 7px 9px;
    border-radius: var(--r-sm);
    background: var(--c-surface-raised);
    border: 1px solid var(--c-hairline);
    font-family: var(--f-mono);
    font-size: var(--t-xs);
    line-height: 1.45;
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    -webkit-user-select: text;
    user-select: text;
  }
</style>

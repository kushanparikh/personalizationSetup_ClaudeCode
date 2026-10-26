<script lang="ts">
  // One message strip at the top of the content area. Text is always interpolated, never HTML.
  type Tone = "danger" | "wait" | "done";
  let {
    tone,
    text,
    onOpenLog,
    onDismiss,
  }: { tone: Tone; text: string; onOpenLog: () => void; onDismiss?: () => void } = $props();
</script>

<div class="banner {tone}" role={tone === "danger" ? "alert" : "status"}>
  <span class="lamp" aria-hidden="true"></span>
  <p class="text">{text}</p>
  <button type="button" class="link" onclick={onOpenLog}>Open log</button>
  {#if onDismiss}
    <button type="button" class="dismiss" aria-label="Dismiss" title="Dismiss" onclick={onDismiss}>
      <svg viewBox="0 0 10 10" width="10" height="10" aria-hidden="true">
        <path d="M1.5 1.5l7 7M8.5 1.5l-7 7" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
      </svg>
    </button>
  {/if}
</div>

<style>
  .banner {
    --lamp: var(--c-lamp-wait);
    --wash: var(--c-wash-wait);
    position: relative;
    display: flex;
    align-items: baseline;
    gap: 12px;
    padding: 9px 10px 9px calc(var(--lamp-w) + 12px);
    background: var(--wash);
    border-bottom: 1px solid var(--c-hairline);
    font-size: var(--t-sm);
    line-height: var(--lh-body);
  }
  .danger {
    --lamp: var(--c-danger);
    --wash: var(--c-wash-danger);
  }
  .done {
    --lamp: var(--c-lamp-done);
    --wash: var(--c-wash-done);
  }
  .lamp {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: var(--lamp-w);
    background: var(--lamp);
  }
  .text {
    margin: 0;
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    -webkit-user-select: text;
    user-select: text;
  }
  .link {
    flex: none;
    padding: 0;
    border: 0;
    background: none;
    font: inherit;
    color: var(--c-focus);
    cursor: pointer;
  }
  .link:hover {
    text-decoration: underline;
  }
  .link:focus-visible,
  .dismiss:focus-visible {
    outline: 2px solid var(--c-focus);
    outline-offset: 2px;
    border-radius: var(--r-sm);
  }
  .dismiss {
    flex: none;
    align-self: center;
    display: grid;
    place-items: center;
    width: 20px;
    height: 20px;
    padding: 0;
    border: 0;
    border-radius: var(--r-sm);
    background: transparent;
    color: var(--c-muted);
    cursor: pointer;
  }
  .dismiss:hover {
    background: var(--c-hairline);
    color: var(--c-ink);
  }
</style>

<script lang="ts">
  import { onMount } from "svelte";
  import { invoke } from "@tauri-apps/api/core";
  import { listen } from "@tauri-apps/api/event";
  import { splitTitle } from "../lib/title";

  type AlertView = { kind: "panel" | "toast"; title: string; message: string; focus: boolean };

  const label = new URLSearchParams(window.location.search).get("label") ?? "";

  let view = $state<AlertView | null>(null);
  // Bumped on every open/update so the lamp pulses once to draw the eye.
  let pulse = $state(0);

  const parts = $derived(splitTitle(view?.title ?? ""));
  const isPanel = $derived(view?.kind === "panel");
  const body = $derived(view?.message || (isPanel ? "" : "Finished working"));
  const footer = $derived(view?.focus ? "Click to go to the session" : "Click to dismiss");

  function show(v: AlertView | null) {
    if (!v) return;
    view = v;
    pulse += 1;
  }

  onMount(() => {
    let unlisten: (() => void) | undefined;
    let gone = false;
    listen<AlertView>("alert-updated", (e) => show(e.payload), {
      target: { kind: "WebviewWindow", label },
    }).then((u) => (gone ? u() : (unlisten = u)));
    invoke<AlertView | null>("alert_get", { label }).then(show);
    return () => {
      gone = true;
      unlisten?.();
    };
  });

  function activate() {
    invoke("alert_click", { label });
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      activate();
    }
  }

  function dismiss(e: MouseEvent) {
    e.stopPropagation();
    invoke("alert_dismiss", { label });
  }
</script>

{#if view}
  <div
    class="alert"
    class:panel={isPanel}
    class:toast={!isPanel}
    role="button"
    tabindex="0"
    title={footer}
    onclick={activate}
    onkeydown={onKey}
  >
    {#key pulse}<span class="lamp" aria-hidden="true"></span>{/key}
    <div class="content">
      <div class="head">
        <span class="project">{parts.project}</span>
        <span class="product">{parts.product}</span>
      </div>
      {#if isPanel}
        <p class="body">{body}</p>
        <p class="footer">{footer}</p>
      {:else}
        <div class="row">
          <p class="body">{body}</p>
          <p class="footer">{footer}</p>
        </div>
      {/if}
    </div>
    {#if isPanel}
      <button class="dismiss" type="button" aria-label="Dismiss" title="Dismiss" onclick={dismiss}>
        <svg viewBox="0 0 10 10" width="10" height="10" aria-hidden="true">
          <path d="M1.5 1.5l7 7M8.5 1.5l-7 7" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" />
        </svg>
      </button>
    {/if}
  </div>
{/if}

<style>
  :global(html),
  :global(body),
  :global(#app) {
    margin: 0;
    width: 100%;
    height: 100%;
    overflow: hidden;
  }
  :global(body) {
    background: var(--c-surface);
    color: var(--c-ink);
    font-family: var(--f-ui);
    -webkit-font-smoothing: antialiased;
    -webkit-user-select: none;
    user-select: none;
  }

  .alert {
    --lamp: var(--c-lamp-wait);
    position: relative;
    box-sizing: border-box;
    display: flex;
    width: 100%;
    height: 100%;
    padding: 10px 12px 10px calc(var(--lamp-w) + 12px);
    border: 1px solid var(--c-hairline);
    background: var(--c-surface);
    cursor: pointer;
    outline: none;
  }
  .alert.toast {
    --lamp: var(--c-lamp-done);
  }
  .alert:hover {
    background: var(--c-hover);
  }
  .alert:focus-visible {
    box-shadow: inset 0 0 0 2px var(--c-focus);
  }

  .lamp {
    position: absolute;
    left: 0;
    top: 0;
    bottom: 0;
    width: var(--lamp-w);
    background: var(--lamp);
    animation: glow 900ms var(--ease-out) 1;
  }
  @keyframes glow {
    0% {
      box-shadow: 0 0 0 0 var(--lamp);
      filter: brightness(1.35);
    }
    35% {
      box-shadow: 2px 0 14px 2px var(--lamp);
    }
    100% {
      box-shadow: 0 0 0 0 transparent;
      filter: none;
    }
  }
  @media (prefers-reduced-motion: reduce) {
    .lamp {
      animation: none;
    }
  }

  .content {
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 2px;
    min-width: 0;
    flex: 1;
  }

  .head {
    display: flex;
    align-items: baseline;
    gap: 8px;
    min-width: 0;
    padding-right: 22px; /* room for the dismiss button */
  }
  .toast .head {
    padding-right: 0;
  }
  .project {
    font-family: var(--f-name);
    font-size: var(--t-md);
    font-weight: var(--w-semibold);
    line-height: var(--lh-tight);
    letter-spacing: -0.01em;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
    min-width: 0;
  }
  .product {
    flex: none;
    font-size: var(--t-xs);
    color: var(--c-muted);
    line-height: var(--lh-tight);
  }

  p {
    margin: 0;
  }
  .body {
    font-size: var(--t-sm);
    line-height: var(--lh-body);
    overflow-wrap: anywhere;
    overflow: hidden;
    display: -webkit-box;
    -webkit-box-orient: vertical;
    -webkit-line-clamp: 2;
    line-clamp: 2;
  }
  .toast .body {
    -webkit-line-clamp: 1;
    line-clamp: 1;
    min-width: 0;
    flex: 1;
  }
  .footer {
    font-size: var(--t-xs);
    line-height: var(--lh-tight);
    color: var(--c-muted);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .row {
    display: flex;
    align-items: baseline;
    gap: 12px;
    min-width: 0;
  }
  .toast .footer {
    flex: none;
  }

  .dismiss {
    position: absolute;
    top: 7px;
    right: 7px;
    display: grid;
    place-items: center;
    width: 22px;
    height: 22px;
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
  .dismiss:focus-visible {
    outline: 2px solid var(--c-focus);
    outline-offset: -2px;
  }
</style>

<script lang="ts">
  import { clipper } from '../../lib/clipper-store.svelte';

  const LOGIN_PAGE_URL = import.meta.env.VITE_LOGIN_PAGE_URL ?? 'https://currents.is/oauth/login';

  type SaveState = 'idle' | 'saving' | 'saved' | 'error';
  let saveState = $state<SaveState>('idle');
  let errorMsg = $state('');
  let selectedCollectionUri = $state('');
  let text = $state('');

  // Reset form state whenever a new image is shown
  $effect(() => {
    if (clipper.visible) {
      saveState = 'idle';
      errorMsg = '';
      text = '';
      selectedCollectionUri = clipper.collections[0]?.uri ?? '';
    }
  });

  function close() {
    document.dispatchEvent(new CustomEvent('currents-clipper-close'));
  }

  function handleKeydown(e: KeyboardEvent) {
    if (e.key === 'Escape') close();
  }

  async function save() {
    if (!selectedCollectionUri) return;
    saveState = 'saving';
    try {
      const response = await browser.runtime.sendMessage({
        type: 'SAVE_IMAGE',
        imgUrl: clipper.imgUrl,
        collectionUri: selectedCollectionUri,
        text: text.trim(),
        originUrl: clipper.originUrl,
      });
      if (response.ok) {
        saveState = 'saved';
        setTimeout(close, 1500);
      } else {
        saveState = 'error';
        errorMsg = response.error ?? 'Unknown error';
      }
    } catch (e) {
      saveState = 'error';
      errorMsg = String(e);
    }
  }
</script>

<svelte:window onkeydown={handleKeydown} />

{#if clipper.visible}
  <div
    class="backdrop"
    role="presentation"
    onclick={close}
  >
    <div
      class="card"
      role="dialog"
      aria-modal="true"
      aria-label="Save to Currents"
      tabindex="-1"
      onclick={(e) => e.stopPropagation()}
      onkeydown={(e) => e.stopPropagation()}
    >
      <button class="close-btn" onclick={close} aria-label="Close">✕</button>

      {#if clipper.authState === 'unauthenticated'}
        <p class="auth-prompt">
          <a href={LOGIN_PAGE_URL} target="_blank" rel="noreferrer">
            Log in to Currents
          </a> to save images.
        </p>
      {:else}
        <img class="preview" src={clipper.imgUrl} alt="Preview" />

        <select
          bind:value={selectedCollectionUri}
          disabled={saveState === 'saving' || saveState === 'saved'}
        >
          {#if clipper.collections.length === 0}
            <option value="">No collections yet</option>
          {/if}
          {#each clipper.collections as col (col.uri)}
            <option value={col.uri}>{col.name}</option>
          {/each}
        </select>

        <input
          type="text"
          placeholder="Add a note (optional)"
          bind:value={text}
          disabled={saveState === 'saving' || saveState === 'saved'}
        />

        {#if saveState === 'idle' || saveState === 'error'}
          <button onclick={save} disabled={!selectedCollectionUri}>
            Save to Currents
          </button>
          {#if saveState === 'error'}
            <p class="error">{errorMsg}</p>
          {/if}
        {:else if saveState === 'saving'}
          <button disabled>Saving…</button>
        {:else if saveState === 'saved'}
          <p class="success">Saved!</p>
        {/if}
      {/if}
    </div>
  </div>
{/if}

<style>
  .backdrop {
    position: fixed;
    inset: 0;
    z-index: 2147483647;
    background: rgba(0, 0, 0, 0.5);
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: system-ui, -apple-system, sans-serif;
    font-size: 14px;
    line-height: 1.5;
  }

  .card {
    background: #fff;
    color: #111;
    border-radius: 12px;
    padding: 24px;
    width: 360px;
    max-width: calc(100vw - 48px);
    position: relative;
    display: flex;
    flex-direction: column;
    gap: 12px;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.24);
  }

  .preview {
    width: 100%;
    max-height: 200px;
    object-fit: contain;
    border-radius: 6px;
    background: #f0f0f0;
  }

  select,
  input[type='text'] {
    width: 100%;
    padding: 8px 12px;
    border: 1px solid #ddd;
    border-radius: 6px;
    font-size: 14px;
    box-sizing: border-box;
    font-family: inherit;
  }

  select:focus,
  input[type='text']:focus {
    outline: 2px solid #0057ff;
    border-color: transparent;
  }

  button:not(.close-btn) {
    padding: 10px;
    background: #0057ff;
    color: #fff;
    border: none;
    border-radius: 6px;
    font-size: 14px;
    font-family: inherit;
    cursor: pointer;
  }

  button:not(.close-btn):hover:not(:disabled) {
    background: #0046d0;
  }

  button:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .close-btn {
    position: absolute;
    top: 12px;
    right: 12px;
    background: none;
    border: none;
    font-size: 16px;
    cursor: pointer;
    color: #888;
    line-height: 1;
    padding: 2px 4px;
  }

  .close-btn:hover {
    color: #111;
  }

  .error {
    color: #c00;
    font-size: 13px;
    margin: 0;
  }

  .success {
    color: #080;
    font-size: 14px;
    margin: 0;
    font-weight: 500;
  }

  .auth-prompt {
    font-size: 14px;
    margin: 0;
  }

  .auth-prompt a {
    color: #0057ff;
    text-decoration: underline;
  }
</style>

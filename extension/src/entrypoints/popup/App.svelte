<script lang="ts">
  import Logo from '../../assets/logo_merged.svelte';

  const CURRENTS_URL = import.meta.env.VITE_CURRENTS_URL ?? 'https://currents.is';
  const LOGIN_PAGE_URL = import.meta.env.VITE_LOGIN_PAGE_URL ?? 'https://currents.is/oauth/login';

  type Status = 'loading' | 'authenticated' | 'unauthenticated';

  interface AuthState {
    status: Status;
    handle?: string;
  }

  let auth = $state<AuthState>({ status: 'loading' });

  $effect(() => {
    loadAuth();
  });

  async function loadAuth() {
    const result = await browser.storage.session.get('authCache');
    const cache = result.authCache as { did: string; handle: string; fetchedAt: number } | undefined;
    if (cache && Date.now() - cache.fetchedAt < 60_000) {
      auth = { status: 'authenticated', handle: cache.handle };
      return;
    }
    try {
      const resp = await fetch(`${CURRENTS_URL}/api/me`, { credentials: 'include' });
      if (resp.ok) {
        const data = await resp.json();
        await browser.storage.session.set({
          authCache: { did: data.did, handle: data.handle, fetchedAt: Date.now() },
        });
        auth = { status: 'authenticated', handle: data.handle };
      } else {
        auth = { status: 'unauthenticated' };
      }
    } catch {
      auth = { status: 'unauthenticated' };
    }
  }
</script>

<main>
  <div class="logo"><Logo /></div>
  {#if auth.status === 'loading'}
    <p class="muted">Checking login…</p>
  {:else if auth.status === 'authenticated'}
    <p>Logged in as <strong>@{auth.handle}</strong></p>
    <a href="{CURRENTS_URL}/save" target="_blank" rel="noreferrer">View your saves →</a>
  {:else}
    <p class="muted">Not logged in.</p>
    <a href={LOGIN_PAGE_URL} target="_blank" rel="noreferrer">Log in to Currents →</a>
  {/if}
</main>

<style>
  main {
    padding: 16px 20px;
    min-width: 200px;
    font-family: system-ui, -apple-system, sans-serif;
    font-size: 14px;
    line-height: 1.5;
  }

  .logo {
    height: 28px;
    margin-bottom: 12px;
  }

  p {
    margin: 0 0 8px;
  }

  .muted {
    color: #888;
  }

  a {
    color: #0057ff;
    text-decoration: none;
  }

  a:hover {
    text-decoration: underline;
  }
</style>

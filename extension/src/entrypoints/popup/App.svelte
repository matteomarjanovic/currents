<script lang="ts">
  import Logo from "../../assets/logo_merged.svelte";

  const CURRENTS_URL =
    import.meta.env.VITE_CURRENTS_URL ?? "https://api.currents.is";
  const LOGIN_PAGE_URL =
    import.meta.env.VITE_LOGIN_PAGE_URL ?? "https://currents.is/oauth/login";
  const CURRENTS_FRONTEND_URL =
    import.meta.env.VITE_CURRENTS_FRONTEND_URL ?? "https://currents.is";

  type Status = "loading" | "authenticated" | "unauthenticated";

  interface AuthState {
    status: Status;
    handle?: string;
  }

  let auth = $state<AuthState>({ status: "loading" });

  $effect(() => {
    loadAuth();
  });

  async function loadAuth() {
    const result = await browser.storage.session.get("authCache");
    const cache = result.authCache as
      | { did: string; handle: string; fetchedAt: number }
      | undefined;
    if (cache && Date.now() - cache.fetchedAt < 60_000) {
      auth = { status: "authenticated", handle: cache.handle };
      return;
    }
    try {
      const resp = await fetch(`${CURRENTS_URL}/api/me`, {
        credentials: "include",
      });
      if (resp.ok) {
        const data = await resp.json();
        await browser.storage.session.set({
          authCache: {
            did: data.did,
            handle: data.handle,
            fetchedAt: Date.now(),
          },
        });
        auth = { status: "authenticated", handle: data.handle };
      } else {
        auth = { status: "unauthenticated" };
      }
    } catch {
      auth = { status: "unauthenticated" };
    }
  }
</script>

<main class="flex min-w-50 flex-col gap-2 px-5 py-4 text-sm">
  <div class="mb-1 h-7"><Logo /></div>
  {#if auth.status === "loading"}
    <p class="text-muted-foreground">Checking login…</p>
  {:else if auth.status === "authenticated"}
    <p>Logged in as <strong>@{auth.handle}</strong></p>
    <a
      class="text-primary underline-offset-4 hover:underline"
      href="{CURRENTS_FRONTEND_URL}/profile/{auth.handle}"
      target="_blank"
      rel="noreferrer">View your saves →</a
    >
  {:else}
    <p class="text-muted-foreground">Not logged in.</p>
    <a
      class="text-primary underline-offset-4 hover:underline"
      href={LOGIN_PAGE_URL}
      target="_blank"
      rel="noreferrer">Log in to Currents →</a
    >
  {/if}
</main>

<script lang="ts">
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { onMount } from 'svelte';

	let user: { did: string; handle: string } | null = $state(null);
	let loading = $state(true);

	onMount(async () => {
		try {
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/me`, { credentials: 'include' });
			if (res.ok) {
				user = await res.json();
			}
		} catch {
			// appview unreachable
		}
		loading = false;
	});
</script>

{#if loading}
	<p>Loading...</p>
{:else if user}
	<p>Logged in as <strong>{user.handle}</strong></p>
	<a href="{PUBLIC_APPVIEW_URL}/oauth/logout">Logout</a>
{:else}
	<p>Not logged in</p>
	<a href="/login">Login</a>
{/if}

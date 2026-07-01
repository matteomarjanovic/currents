<script lang="ts">
	import '../layout.css';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { ModeWatcher } from 'mode-watcher';

	let { children } = $props();

	let role = $state<string | null>(null);
	let checked = $state(false);

	onMount(async () => {
		try {
			const res = await apiFetch(`/api/me/role`);
			if (res.ok) {
				const data = (await res.json()) as { role: string | null };
				role = data.role;
			}
		} catch {
			// appview unreachable; treat as no role
		}
		checked = true;
		if (!role) {
			goto('/');
		}
	});
</script>

<ModeWatcher />

{#if !checked}
	<div class="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
		Loading…
	</div>
{:else if !role}
	<div class="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
		Redirecting…
	</div>
{:else}
	<div class="min-h-screen">
		<header class="sticky top-0 z-10 border-b border-border bg-background/95 backdrop-blur">
			<div class="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
				<a href="/" class="text-sm font-semibold tracking-tight">Currents · stats</a>
				<span class="text-xs text-muted-foreground">{role}</span>
			</div>
		</header>
		<main class="mx-auto max-w-6xl px-4 py-6">
			{@render children()}
		</main>
	</div>
{/if}

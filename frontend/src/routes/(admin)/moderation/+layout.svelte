<script lang="ts">
	import '../../layout.css';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import { ModeWatcher } from 'mode-watcher';
	import { Toaster } from '$lib/components/ui/sonner';

	let { children } = $props();

	let moderatorRole = $state<string | null>(null);
	let checked = $state(false);

	onMount(async () => {
		try {
			const res = await apiFetch(`/api/me/role`);
			if (res.ok) {
				const data = (await res.json()) as { moderatorRole: string | null };
				moderatorRole = data.moderatorRole;
			}
		} catch {
			// appview unreachable; treat as no role
		}
		checked = true;
		if (!moderatorRole) {
			goto('/');
		}
	});

	const navLinks = [
		{ href: '/moderation/queue', label: 'Queue' },
		{ href: '/moderation/history', label: 'History' }
	];
</script>

<ModeWatcher />

{#if !checked}
	<div class="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
		Loading…
	</div>
{:else if !moderatorRole}
	<div class="flex min-h-screen items-center justify-center text-sm text-muted-foreground">
		Redirecting…
	</div>
{:else}
	<div class="min-h-screen">
		<header class="sticky top-0 z-10 border-b border-border bg-background/95 backdrop-blur">
			<div class="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
				<div class="flex items-center gap-4">
					<a href="/moderation/queue" class="text-sm font-semibold tracking-tight"
						>Currents · moderation</a
					>
					<nav class="flex items-center gap-1 text-sm">
						{#each navLinks as link (link.href)}
							{@const active = page.url.pathname.startsWith(link.href)}
							<a
								href={link.href}
								class="rounded-md px-2 py-1 transition-colors hover:bg-muted {active
									? 'bg-muted font-medium'
									: 'text-muted-foreground'}"
							>
								{link.label}
							</a>
						{/each}
					</nav>
				</div>
				<span class="text-xs text-muted-foreground">{moderatorRole}</span>
			</div>
		</header>
		<main class="mx-auto max-w-6xl px-4 py-6">
			{@render children()}
		</main>
	</div>
{/if}

<Toaster />

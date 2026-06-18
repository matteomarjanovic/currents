<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { PUBLIC_APPVIEW_URL } from '$env/static/public';
	import { useInfiniteScroll } from '$lib/hooks/use-infinite-scroll.svelte';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import UserIcon from '@lucide/svelte/icons/user';
	import type { ActorProfileView } from '$lib/types';

	let {
		endpoint,
		listKey,
		did,
		emptyText = 'Nobody here yet.',
		onNavigate = () => {}
	}: {
		endpoint: string;
		listKey: string;
		did: string;
		emptyText?: string;
		onNavigate?: () => void;
	} = $props();

	const list = useInfiniteScroll<ActorProfileView>(
		async (cursor) => {
			const params = new URLSearchParams({ actor: did, limit: '50' });
			if (cursor) params.set('cursor', cursor);
			const res = await fetch(`${PUBLIC_APPVIEW_URL}/xrpc/${endpoint}?${params}`, {
				credentials: 'include'
			});
			const data = await res.json();
			return { items: data[listKey] ?? [], cursor: data.cursor };
		},
		(a) => a.did
	);

	let scrollEl: HTMLDivElement = $state(undefined!);
	let sentinel: HTMLDivElement | undefined = $state();

	onMount(() => list.loadMore());

	$effect(() => {
		if (!sentinel || !scrollEl) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) list.loadMore();
			},
			{ root: scrollEl, rootMargin: '200px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});
</script>

<div bind:this={scrollEl} class="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-3 pb-3">
	{#each list.items as actor (actor.did)}
		<a
			href={resolve('/(with-navbar)/profile/[handle]', { handle: actor.handle })}
			onclick={onNavigate}
			class="flex items-center gap-3 rounded-lg p-2 transition-colors hover:bg-muted"
		>
			<Avatar.Root class="size-11 shrink-0">
				{#if actor.avatar}
					<Avatar.Image src={actor.avatar} alt={actor.displayName ?? actor.handle} />
				{/if}
				<Avatar.Fallback>
					<UserIcon class="size-5" />
				</Avatar.Fallback>
			</Avatar.Root>
			<div class="min-w-0">
				<div class="truncate font-medium text-foreground">
					{actor.displayName ?? actor.handle}
				</div>
				<div class="truncate text-sm text-muted-foreground">@{actor.handle}</div>
			</div>
		</a>
	{/each}

	{#if list.loading}
		{#each [0, 1, 2, 3] as i (i)}
			<div class="flex items-center gap-3 p-2">
				<Skeleton class="size-11 shrink-0 rounded-full" />
				<div class="flex-1 space-y-2">
					<Skeleton class="h-4 w-32" />
					<Skeleton class="h-3 w-24" />
				</div>
			</div>
		{/each}
	{:else if list.items.length === 0}
		<p class="py-10 text-center text-sm text-muted-foreground">{emptyText}</p>
	{/if}

	{#if list.hasMore}
		<div bind:this={sentinel} class="h-1"></div>
	{/if}
</div>

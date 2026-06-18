<script lang="ts">
	import { resolve } from '$app/paths';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import UserIcon from '@lucide/svelte/icons/user';
	import { social, loadMoreSocial, type FollowerNotification } from '$lib/stores/social.svelte';
	import { followUser, unfollowUser } from '$lib/follow';

	let {
		onNavigate = () => {},
		onScopeMissing = () => {}
	}: {
		onNavigate?: () => void;
		onScopeMissing?: () => void;
	} = $props();

	let busyDid = $state<string | null>(null);
	let scrollEl: HTMLDivElement = $state(undefined!);
	let sentinel: HTMLDivElement | undefined = $state();

	$effect(() => {
		if (!sentinel || !scrollEl) return;
		const observer = new IntersectionObserver(
			(entries) => {
				if (entries[0].isIntersecting) loadMoreSocial();
			},
			{ root: scrollEl, rootMargin: '200px' }
		);
		observer.observe(sentinel);
		return () => observer.disconnect();
	});

	async function toggleFollow(item: FollowerNotification) {
		busyDid = item.did;
		try {
			if (item.youFollow) {
				if (item.followUri && (await unfollowUser(item.followUri))) {
					item.youFollow = false;
					item.followUri = undefined;
				}
			} else {
				const out = await followUser(item.did);
				if (out.status === 'ok') {
					item.youFollow = true;
					item.followUri = out.uri;
				} else if (out.status === 'scope-missing') {
					onScopeMissing();
				}
			}
		} finally {
			busyDid = null;
		}
	}

	function timeAgo(iso: string): string {
		const s = Math.floor((Date.now() - new Date(iso).getTime()) / 1000);
		if (s < 60) return 'now';
		const m = Math.floor(s / 60);
		if (m < 60) return `${m}m`;
		const h = Math.floor(m / 60);
		if (h < 24) return `${h}h`;
		const d = Math.floor(h / 24);
		if (d < 7) return `${d}d`;
		const w = Math.floor(d / 7);
		if (w < 5) return `${w}w`;
		return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
	}
</script>

<div bind:this={scrollEl} class="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-3 pb-3">
	{#each social.items as item (item.did)}
		<div class="flex items-center gap-3 rounded-lg p-2 {item.isNew ? 'bg-accent/40' : ''}">
			<a
				href={resolve('/(with-navbar)/profile/[handle]', { handle: item.handle })}
				onclick={onNavigate}
				class="flex min-w-0 flex-1 items-center gap-3"
			>
				<Avatar.Root class="size-11 shrink-0">
					{#if item.avatar}
						<Avatar.Image src={item.avatar} alt={item.displayName ?? item.handle} />
					{/if}
					<Avatar.Fallback>
						<UserIcon class="size-5" />
					</Avatar.Fallback>
				</Avatar.Root>
				<div class="flex min-w-0 flex-col items-start justify-center gap-2">
					<div class="truncate text-sm">
						<span class="font-medium text-foreground">{item.displayName ?? item.handle}</span>
						<span class="text-muted-foreground">
							followed you&ensp;•&ensp;{timeAgo(item.followedAt)}</span
						>
					</div>
					<Button
						size="sm"
						variant={item.youFollow ? 'secondary' : 'default'}
						class="shrink-0 rounded-full"
						onclick={() => toggleFollow(item)}
						disabled={busyDid === item.did}
					>
						{item.youFollow ? 'Following' : '+ Follow back'}
					</Button>
					<!-- <div class="truncate text-xs text-muted-foreground">{timeAgo(item.followedAt)}</div> -->
				</div>
			</a>
			<!-- <Button
				size="sm"
				variant={item.youFollow ? 'secondary' : 'default'}
				class="shrink-0 rounded-full"
				onclick={() => toggleFollow(item)}
				disabled={busyDid === item.did}
			>
				{item.youFollow ? 'Following' : 'Follow back'}
			</Button> -->
			<!-- <div class="truncate text-xs text-muted-foreground">{timeAgo(item.followedAt)}</div> -->
		</div>
	{/each}

	{#if social.loading && social.items.length === 0}
		{#each [0, 1, 2, 3] as i (i)}
			<div class="flex items-center gap-3 p-2">
				<Skeleton class="size-11 shrink-0 rounded-full" />
				<div class="flex-1 space-y-2">
					<Skeleton class="h-4 w-40" />
					<Skeleton class="h-3 w-24" />
				</div>
			</div>
		{/each}
	{:else if social.error}
		<p class="py-10 text-center text-sm text-destructive">{social.error}</p>
	{:else if social.items.length === 0}
		<p class="py-10 text-center text-sm text-muted-foreground">
			When someone follows you, they'll show up here.
		</p>
	{/if}

	{#if social.hasMore}
		<div bind:this={sentinel} class="h-1"></div>
	{/if}
</div>

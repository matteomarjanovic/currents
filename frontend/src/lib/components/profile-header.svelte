<script lang="ts">
	import type { ActorProfileView } from '$lib/types';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import UserIcon from '@lucide/svelte/icons/user';
	import LinkIcon from '@lucide/svelte/icons/link';
	import Pencil from '@lucide/svelte/icons/pencil';
	import FollowListDialog from './follow-list-dialog.svelte';
	import FollowScopeDialog from './follow-scope-dialog.svelte';
	import { followUser, unfollowUser } from '$lib/follow';

	let {
		profile,
		isOwner = false,
		onEdit = () => {}
	}: {
		profile: ActorProfileView;
		isOwner?: boolean;
		onEdit?: () => void;
	} = $props();

	const subjectDid = $derived(profile.did);
	const initialFollowing = $derived(!!profile.viewer?.following);
	const initialFollowUri = $derived(profile.viewer?.following ?? '');

	let following = $state(false);
	let followUri = $state('');
	let followLoading = $state(false);
	let scopeMissing = $state(false);

	// Local follower count so following/unfollowing updates the number immediately.
	let followersCount = $state(0);

	let listOpen = $state(false);
	let listTab = $state<'followers' | 'following'>('followers');

	$effect(() => {
		following = initialFollowing;
		followUri = initialFollowUri;
		followersCount = profile.followersCount ?? 0;
	});

	function openList(tab: 'followers' | 'following') {
		listTab = tab;
		listOpen = true;
	}

	async function toggleFollow() {
		followLoading = true;
		try {
			if (following) {
				if (followUri && (await unfollowUser(followUri))) {
					following = false;
					followUri = '';
					followersCount = Math.max(0, followersCount - 1);
				}
			} else {
				const out = await followUser(subjectDid);
				if (out.status === 'ok') {
					following = true;
					followUri = out.uri;
					followersCount = followersCount + 1;
				} else if (out.status === 'scope-missing') {
					scopeMissing = true;
				}
			}
		} finally {
			followLoading = false;
		}
	}

	const initials = $derived(
		(profile.displayName ?? profile.handle ?? '?').trim().charAt(0).toUpperCase()
	);

	function websiteLabel(url: string) {
		try {
			return new URL(url).hostname.replace(/^www\./, '');
		} catch {
			return url;
		}
	}
</script>

<section class="mb-6">
	{#if profile.banner}
		<div class="relative h-40 w-full overflow-hidden rounded-xl bg-muted sm:h-56">
			<img src={profile.banner} alt="" class="h-full w-full object-cover" />
		</div>
	{/if}

	<div
		class="flex w-full flex-col gap-4 px-1 sm:flex-row sm:items-end sm:justify-between {profile.banner
			? '-mt-10 sm:-mt-12'
			: 'mt-2'}"
	>
		<div class="flex min-w-0 flex-1 items-end gap-4">
			<Avatar.Root class="size-24 border-4 border-background sm:size-28">
				{#if profile.avatar}
					<Avatar.Image src={profile.avatar} alt={profile.displayName ?? profile.handle} />
				{/if}
				<Avatar.Fallback class="text-2xl">
					{#if initials && initials !== '?'}
						{initials}
					{:else}
						<UserIcon class="size-8" />
					{/if}
				</Avatar.Fallback>
			</Avatar.Root>

			<div class="min-w-0 pb-1">
				<h1 class="truncate text-2xl font-semibold text-foreground">
					{profile.displayName ?? profile.handle}
				</h1>
				<div class="truncate text-sm text-muted-foreground">@{profile.handle}</div>
			</div>
		</div>

		{#if isOwner}
			<Button type="button" variant="outline" size="sm" class="rounded-full" onclick={onEdit}>
				<Pencil class="size-4" />
				Edit profile
			</Button>
		{/if}
	</div>

	<div class="mt-4 flex flex-wrap items-center gap-x-5 gap-y-2 px-1">
		<button
			type="button"
			class="text-sm transition-colors hover:text-foreground hover:underline"
			onclick={() => openList('followers')}
		>
			<span class="font-semibold text-foreground">{followersCount}</span>
			<span class="text-muted-foreground">
				{followersCount === 1 ? 'follower' : 'followers'}
			</span>
		</button>
		<button
			type="button"
			class="text-sm transition-colors hover:text-foreground hover:underline"
			onclick={() => openList('following')}
		>
			<span class="font-semibold text-foreground">{profile.followsCount ?? 0}</span>
			<span class="text-muted-foreground">following</span>
		</button>
		{#if !isOwner}
			<Button
				type="button"
				variant={following ? 'secondary' : 'default'}
				size="sm"
				class="shrink-0 rounded-full"
				onclick={toggleFollow}
				disabled={followLoading}
			>
				{following ? 'Following' : 'Follow'}
			</Button>
		{/if}
	</div>

	{#if profile.description || profile.pronouns || profile.website}
		<div class="mt-4 space-y-2 px-1">
			{#if profile.pronouns}
				<span
					class="inline-block rounded-full border bg-muted px-2 py-0.5 text-xs text-muted-foreground"
				>
					{profile.pronouns}
				</span>
			{/if}
			{#if profile.description}
				<p class="text-sm whitespace-pre-wrap text-foreground">{profile.description}</p>
			{/if}
			{#if profile.website}
				<Button
					type="button"
					variant="link"
					class="inline-flex h-auto items-center gap-1 px-0 py-0 text-sm text-primary hover:underline"
					onclick={() => window.open(profile.website, '_blank', 'noopener,noreferrer')}
				>
					<LinkIcon class="size-3.5" />
					{websiteLabel(profile.website)}
				</Button>
			{/if}
		</div>
	{/if}

	<Separator class="mt-6" />
</section>

<FollowListDialog
	bind:open={listOpen}
	bind:tab={listTab}
	did={profile.did}
	name={profile.displayName ?? profile.handle}
	{followersCount}
	followsCount={profile.followsCount ?? 0}
/>

<FollowScopeDialog bind:open={scopeMissing} />

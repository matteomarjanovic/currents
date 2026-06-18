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
	import BlueskyFollowImportDialog from './bluesky-follow-import-dialog.svelte';
	import { followUser, unfollowUser } from '$lib/follow';
	import {
		features,
		isFeatureSeen,
		markFeatureSeen,
		FEATURE_BLUESKY_IMPORT
	} from '$lib/stores/features.svelte';

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
	let importOpen = $state(false);

	// One-time "new feature" dot on the Bluesky-import button; gated on `loaded`
	// so it never flashes before we know what the user has already dismissed.
	const showImportNew = $derived(features.loaded && !isFeatureSeen(FEATURE_BLUESKY_IMPORT));

	function openImport() {
		markFeatureSeen(FEATURE_BLUESKY_IMPORT);
		importOpen = true;
	}

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
		{:else}
			<Button
				type="button"
				variant="outline"
				size="sm"
				class="relative shrink-0 rounded-full"
				onclick={openImport}
			>
				<svg viewBox="0 0 568 501" class="size-4 shrink-0" aria-hidden="true">
					<path
						fill="#1185fe"
						d="M123.121 33.664C188.241 82.553 258.281 181.68 284 234.873c25.719-53.193 95.759-152.32 160.879-201.21C491.866-1.611 568-28.906 568 57.947c0 17.346-9.945 145.713-15.778 166.555-20.275 72.453-94.155 90.933-159.875 79.748C507.222 323.8 536.444 388.56 473.333 453.32c-119.86 122.992-172.272-30.859-185.702-70.281-2.462-7.227-3.614-10.608-3.631-7.733-.017-2.875-1.169.506-3.631 7.733-13.43 39.422-65.842 193.273-185.702 70.281-63.111-64.76-33.889-129.52 80.986-149.071-65.72 11.185-139.6-7.295-159.875-79.748C9.945 203.66 0 75.293 0 57.947 0-28.906 76.135-1.611 123.121 33.664Z"
					/>
				</svg>
				Find Bluesky friends
				{#if showImportNew}
					<span
						class="absolute -top-1 -right-1 inline-flex h-2.5 w-2.5 rounded-full bg-red-500 ring-2 ring-background"
						aria-hidden="true"
					></span>
				{/if}
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

{#if isOwner}
	<BlueskyFollowImportDialog bind:open={importOpen} />
{/if}

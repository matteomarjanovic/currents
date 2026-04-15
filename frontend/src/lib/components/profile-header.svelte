<script lang="ts">
	import type { ActorProfileView } from '$lib/types';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Separator } from '$lib/components/ui/separator';
	import UserIcon from '@lucide/svelte/icons/user';
	import LinkIcon from '@lucide/svelte/icons/link';

	let { profile }: { profile: ActorProfileView } = $props();

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
		class="flex flex-col items-start gap-4 px-1 sm:flex-row sm:items-end {profile.banner
			? '-mt-10 sm:-mt-12'
			: 'mt-2'}"
	>
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

		<div class="min-w-0 flex-1 pb-1">
			<h1 class="truncate text-2xl font-semibold text-foreground">
				{profile.displayName ?? profile.handle}
			</h1>
			<div class="truncate text-sm text-muted-foreground">@{profile.handle}</div>
		</div>
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
				<p class="whitespace-pre-wrap text-sm text-foreground">{profile.description}</p>
			{/if}
			{#if profile.website}
				<a
					href={profile.website}
					target="_blank"
					rel="noreferrer noopener"
					class="inline-flex items-center gap-1 text-sm text-primary hover:underline"
				>
					<LinkIcon class="size-3.5" />
					{websiteLabel(profile.website)}
				</a>
			{/if}
		</div>
	{/if}

	<Separator class="mt-6" />
</section>

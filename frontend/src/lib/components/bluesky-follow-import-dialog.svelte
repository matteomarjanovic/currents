<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Avatar from '$lib/components/ui/avatar';
	import { Button } from '$lib/components/ui/button';
	import { Skeleton } from '$lib/components/ui/skeleton';
	import UserIcon from '@lucide/svelte/icons/user';
	import { resolve } from '$app/paths';
	import { apiFetch } from '$lib/api';
	import { followUser } from '$lib/follow';
	import FollowScopeDialog from './follow-scope-dialog.svelte';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	type Candidate = {
		did: string;
		handle: string;
		displayName?: string;
		avatar?: string;
		following: boolean;
		busy: boolean;
	};

	let loading = $state(false);
	let error = $state(false);
	let candidates = $state<Candidate[]>([]);
	let followingAll = $state(false);
	let scopeMissing = $state(false);

	const remaining = $derived(candidates.filter((c) => !c.following).length);

	async function load() {
		loading = true;
		error = false;
		try {
			const res = await apiFetch(`/api/me/bluesky-follows`);
			if (!res.ok) throw new Error('request failed');
			const data = await res.json();
			candidates = (data.actors ?? []).map((a: Omit<Candidate, 'following' | 'busy'>) => ({
				...a,
				following: false,
				busy: false
			}));
		} catch {
			error = true;
			candidates = [];
		} finally {
			loading = false;
		}
	}

	// Fetch each time the dialog opens so already-followed users drop off; reset on close.
	$effect(() => {
		if (open) {
			load();
		} else {
			candidates = [];
			error = false;
		}
	});

	async function follow(c: Candidate) {
		if (c.busy || c.following) return;
		c.busy = true;
		const out = await followUser(c.did);
		c.busy = false;
		if (out.status === 'ok') c.following = true;
		else if (out.status === 'scope-missing') scopeMissing = true;
	}

	// Follow everyone not yet followed, one at a time — each is a PDS write, so
	// sequential avoids hammering the user's repo.
	async function followAll() {
		if (followingAll) return;
		followingAll = true;
		for (const c of candidates) {
			if (c.following) continue;
			const out = await followUser(c.did);
			if (out.status === 'ok') c.following = true;
			else if (out.status === 'scope-missing') {
				scopeMissing = true;
				break;
			}
		}
		followingAll = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="flex h-5/6 flex-col gap-0 overflow-hidden p-0 pt-6 sm:max-w-md">
		<Dialog.Header class="px-6 pb-3 text-left">
			<Dialog.Title class="flex items-center gap-2 pr-8 text-base">
				<svg viewBox="0 0 568 501" class="size-4 shrink-0" aria-hidden="true">
					<path
						fill="#1185fe"
						d="M123.121 33.664C188.241 82.553 258.281 181.68 284 234.873c25.719-53.193 95.759-152.32 160.879-201.21C491.866-1.611 568-28.906 568 57.947c0 17.346-9.945 145.713-15.778 166.555-20.275 72.453-94.155 90.933-159.875 79.748C507.222 323.8 536.444 388.56 473.333 453.32c-119.86 122.992-172.272-30.859-185.702-70.281-2.462-7.227-3.614-10.608-3.631-7.733-.017-2.875-1.169.506-3.631 7.733-13.43 39.422-65.842 193.273-185.702 70.281-63.111-64.76-33.889-129.52 80.986-149.071-65.72 11.185-139.6-7.295-159.875-79.748C9.945 203.66 0 75.293 0 57.947 0-28.906 76.135-1.611 123.121 33.664Z"
					/>
				</svg>
				<span class="min-w-0 truncate">Find friends from Bluesky</span>
			</Dialog.Title>
			<Dialog.Description>
				People you follow on Bluesky who are also on Currents.
			</Dialog.Description>
		</Dialog.Header>

		{#if remaining > 1 && !loading}
			<div class="px-6 pb-3">
				<Button size="sm" class="rounded-full" onclick={followAll} disabled={followingAll}>
					{followingAll ? 'Following…' : `Follow all (${remaining})`}
				</Button>
			</div>
		{/if}

		<div class="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-3 pb-3">
			{#if loading}
				{#each [0, 1, 2, 3] as i (i)}
					<div class="flex items-center gap-3 p-2">
						<Skeleton class="size-11 shrink-0 rounded-full" />
						<div class="flex-1 space-y-2">
							<Skeleton class="h-4 w-32" />
							<Skeleton class="h-3 w-24" />
						</div>
					</div>
				{/each}
			{:else if error}
				<p class="py-10 text-center text-sm text-muted-foreground">
					Couldn't load your Bluesky follows. Try again in a moment.
				</p>
			{:else if candidates.length === 0}
				<p class="py-10 text-center text-sm text-muted-foreground">
					No one new to follow — everyone you follow on Bluesky is either already followed here or
					not on Currents yet.
				</p>
			{:else}
				{#each candidates as c (c.did)}
					<div class="flex items-center gap-3 rounded-lg p-2 transition-colors hover:bg-muted">
						<a
							href={resolve('/(with-navbar)/profile/[handle]', { handle: c.handle })}
							onclick={() => (open = false)}
							class="flex min-w-0 flex-1 items-center gap-3"
						>
							<Avatar.Root class="size-11 shrink-0">
								{#if c.avatar}
									<Avatar.Image src={c.avatar} alt={c.displayName ?? c.handle} />
								{/if}
								<Avatar.Fallback>
									<UserIcon class="size-5" />
								</Avatar.Fallback>
							</Avatar.Root>
							<div class="min-w-0">
								<div class="truncate font-medium text-foreground">
									{c.displayName ?? c.handle}
								</div>
								<div class="truncate text-sm text-muted-foreground">@{c.handle}</div>
							</div>
						</a>
						<Button
							variant={c.following ? 'secondary' : 'default'}
							size="sm"
							class="shrink-0 rounded-full"
							disabled={c.busy || c.following || followingAll}
							onclick={() => follow(c)}
						>
							{c.following ? 'Following' : 'Follow'}
						</Button>
					</div>
				{/each}
			{/if}
		</div>
	</Dialog.Content>
</Dialog.Root>

<FollowScopeDialog bind:open={scopeMissing} />

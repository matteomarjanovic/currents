<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Tabs from '$lib/components/ui/tabs';
	import Users from '@lucide/svelte/icons/users';
	import ActorList from './actor-list.svelte';

	// `tab` is owned by the parent: it sets which tab to open (followers/following)
	// when the count is clicked, and stays in sync as the user switches tabs.
	let {
		open = $bindable(false),
		tab = $bindable('followers'),
		did,
		name,
		followersCount = 0,
		followsCount = 0
	}: {
		open?: boolean;
		tab?: 'followers' | 'following';
		did: string;
		name: string;
		followersCount?: number;
		followsCount?: number;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="flex h-5/6 flex-col gap-0 overflow-hidden p-0 pt-6 sm:max-w-md">
		<Dialog.Header class="px-6 pb-3 text-left">
			<Dialog.Title class="flex items-center gap-2 pr-8 text-base">
				<Users class="size-4 shrink-0 text-muted-foreground" />
				<span class="min-w-0 truncate">{name}'s connections</span>
			</Dialog.Title>
		</Dialog.Header>
		<Tabs.Root bind:value={tab} class="flex min-h-0 w-full flex-1 gap-0">
			<Tabs.List variant="line" class="grid w-full shrink-0 grid-cols-2 rounded-none">
				<Tabs.Trigger value="followers" class="">
					{followersCount} Followers
				</Tabs.Trigger>
				<Tabs.Trigger value="following" class="">
					{followsCount} Following
				</Tabs.Trigger>
			</Tabs.List>
			<Tabs.Content value="followers" class="mt-3 flex min-h-0 flex-col">
				<ActorList
					endpoint="is.currents.graph.getFollowers"
					listKey="followers"
					{did}
					emptyText="No followers yet."
					onNavigate={() => (open = false)}
				/>
			</Tabs.Content>
			<Tabs.Content value="following" class="mt-3 flex min-h-0 flex-col">
				<ActorList
					endpoint="is.currents.graph.getFollows"
					listKey="follows"
					{did}
					emptyText="Not following anyone yet."
					onNavigate={() => (open = false)}
				/>
			</Tabs.Content>
		</Tabs.Root>
	</Dialog.Content>
</Dialog.Root>

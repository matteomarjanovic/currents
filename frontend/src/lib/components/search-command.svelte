<script lang="ts">
	import { untrack } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import * as Command from '$lib/components/ui/command';
	import ImageIcon from '@lucide/svelte/icons/image';
	import Folder from '@lucide/svelte/icons/folder';
	import UsersIcon from '@lucide/svelte/icons/users';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	const TYPES = [
		{ value: 'saves', label: 'images', icon: ImageIcon },
		{ value: 'collections', label: 'collections', icon: Folder },
		{ value: 'users', label: 'users', icon: UsersIcon }
	] as const;

	let query = $state('');
	let trimmed = $derived(query.trim());

	// Reseed from the current search page each time the dialog opens, so refining
	// an active search starts from its query.
	let prevOpen = false;
	$effect(() => {
		const isOpen = open;
		untrack(() => {
			if (isOpen && !prevOpen) query = page.params.query ?? '';
			prevOpen = isOpen;
		});
	});

	function submit(type: (typeof TYPES)[number]['value']) {
		if (!trimmed) return;
		goto(
			resolve('/(with-navbar)/search/[type]/[query]', {
				type,
				query: encodeURIComponent(trimmed)
			})
		);
		open = false;
	}
</script>

<Command.Dialog
	bind:open
	shouldFilter={false}
	title="Search"
	description="Search images, collections, or users."
>
	<Command.Input bind:value={query} placeholder="Search…" />
	<Command.List>
		<Command.Group>
			{#each TYPES as t (t.value)}
				<Command.Item value={t.value} disabled={!trimmed} onSelect={() => submit(t.value)}>
					<t.icon />
					<span class="truncate">
						Search {t.label}{trimmed ? ` for “${trimmed}”` : ''}
					</span>
				</Command.Item>
			{/each}
		</Command.Group>
	</Command.List>
</Command.Dialog>

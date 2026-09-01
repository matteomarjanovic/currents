<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import * as ContextMenu from '$lib/components/ui/context-menu';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import ReportDialog from '$lib/components/report-dialog.svelte';
	import { copyImage, copyLink, downloadImage, shareLink } from '$lib/save-actions';
	import { hideFeedImage } from '$lib/stores/hidden-feed-images.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import type { SaveView } from '$lib/types';
	import Copy from '@lucide/svelte/icons/copy';
	import Download from '@lucide/svelte/icons/download';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import EyeOff from '@lucide/svelte/icons/eye-off';
	import Flag from '@lucide/svelte/icons/flag';
	import LinkIcon from '@lucide/svelte/icons/link';
	import Share2 from '@lucide/svelte/icons/share-2';

	let {
		item,
		variant,
		contextDisabled = false,
		children,
		onReport
	}: {
		item: SaveView;
		variant: 'context' | 'dropdown';
		contextDisabled?: boolean;
		children?: Snippet;
		onReport?: () => void;
	} = $props();

	const dropdownMenu = DropdownMenu as unknown as typeof ContextMenu;
	let reportOpen = $state(false);

	function run(action: (save: SaveView) => Promise<void>) {
		void action(item);
	}

	function report() {
		if (!auth.user) {
			promptLogin();
			return;
		}
		if (onReport) onReport();
		else reportOpen = true;
	}
</script>

{#snippet menuItems(Menu: typeof ContextMenu)}
	<Menu.Item onSelect={() => run(downloadImage)}>
		<Download />
		Download
	</Menu.Item>
	<Menu.Item onSelect={() => run(copyImage)}>
		<Copy />
		Copy image
	</Menu.Item>
	<Menu.Item onSelect={() => run(shareLink)}>
		<Share2 />
		Share
	</Menu.Item>
	<Menu.Item onSelect={() => run(copyLink)}>
		<LinkIcon />
		Copy link
	</Menu.Item>
	<Menu.Separator />
	<Menu.Item onSelect={() => void hideFeedImage(item)}>
		<EyeOff />
		Hide
	</Menu.Item>
	<Menu.Item onSelect={report}>
		<Flag />
		Report
	</Menu.Item>
{/snippet}

{#if variant === 'context'}
	<ContextMenu.Root>
		<ContextMenu.Trigger disabled={contextDisabled}>
			{#snippet child({ props })}
				<div {...props}>{@render children?.()}</div>
			{/snippet}
		</ContextMenu.Trigger>
		<ContextMenu.Content class="w-52">
			{@render menuItems(ContextMenu)}
		</ContextMenu.Content>
	</ContextMenu.Root>
{:else}
	<DropdownMenu.Root>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant="outline" size="icon-sm" aria-label="Image actions">
					<Ellipsis class="size-4" />
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		<DropdownMenu.Content align="end" class="w-52">
			{@render menuItems(dropdownMenu)}
		</DropdownMenu.Content>
	</DropdownMenu.Root>
{/if}

{#if !onReport && reportOpen}
	<ReportDialog bind:open={reportOpen} save={item} />
{/if}

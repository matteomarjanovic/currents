<script lang="ts">
	import type { Snippet } from 'svelte';
	import { Button, type ButtonVariant } from '$lib/components/ui/button';
	import * as ContextMenu from '$lib/components/ui/context-menu';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import ReportDialog from '$lib/components/report-dialog.svelte';
	import { copyImage, copyLink, downloadImage, shareLink, saveLink } from '$lib/save-actions';
	import { hideFeedImage } from '$lib/stores/hidden-feed-images.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { promptLogin } from '$lib/stores/login-prompt.svelte';
	import { isNative } from '$lib/platform';
	import type { SaveView } from '$lib/types';
	import Scan from '@lucide/svelte/icons/scan';
	import Copy from '@lucide/svelte/icons/copy';
	import Download from '@lucide/svelte/icons/download';
	import Ellipsis from '@lucide/svelte/icons/ellipsis';
	import EyeOff from '@lucide/svelte/icons/eye-off';
	import Flag from '@lucide/svelte/icons/flag';
	import LinkIcon from '@lucide/svelte/icons/link';
	import Share2 from '@lucide/svelte/icons/share-2';
	import ExternalLink from '@lucide/svelte/icons/external-link';

	let {
		item,
		variant,
		contextDisabled = false,
		children,
		onReport,
		onViewFullScreen,
		onOpenChange,
		onBusyChange,
		buttonVariant = 'outline'
	}: {
		item: SaveView;
		variant: 'context' | 'dropdown';
		contextDisabled?: boolean;
		children?: Snippet;
		onReport?: () => void;
		onViewFullScreen?: (save: SaveView) => void;
		onOpenChange?: (open: boolean) => void;
		onBusyChange?: (busy: boolean) => void;
		buttonVariant?: ButtonVariant;
	} = $props();

	const native = isNative();
	const dropdownMenu = DropdownMenu as unknown as typeof ContextMenu;
	let menuOpen = $state(false);
	let reportOpen = $state(false);
	$effect(() => {
		onOpenChange?.(menuOpen);
		onBusyChange?.(menuOpen || reportOpen);
	});

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
	{#if onViewFullScreen}
		<Menu.Item onSelect={() => onViewFullScreen(item)}>
			<Scan />
			View full screen
		</Menu.Item>
		<Menu.Separator />
	{/if}
	{#if !native}
		<Menu.Item onSelect={() => window.open(saveLink(item), '_blank', 'noopener,noreferrer')}>
			<ExternalLink />
			Open in new tab
		</Menu.Item>
	{/if}
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

{#if variant === 'context' && !contextDisabled}
	<ContextMenu.Root bind:open={menuOpen}>
		<ContextMenu.Trigger>
			{#snippet child({ props })}
				<div {...props}>{@render children?.()}</div>
			{/snippet}
		</ContextMenu.Trigger>
		{#if menuOpen}
			<ContextMenu.Content class="w-52">
				{@render menuItems(ContextMenu)}
			</ContextMenu.Content>
		{/if}
	</ContextMenu.Root>
{:else if variant === 'context'}
	{@render children?.()}
{:else}
	<DropdownMenu.Root bind:open={menuOpen}>
		<DropdownMenu.Trigger>
			{#snippet child({ props })}
				<Button {...props} variant={buttonVariant} size="icon-sm" aria-label="Image actions">
					<Ellipsis class="size-4" />
				</Button>
			{/snippet}
		</DropdownMenu.Trigger>
		{#if menuOpen}
			<DropdownMenu.Content align="end" class="w-52">
				{@render menuItems(dropdownMenu)}
			</DropdownMenu.Content>
		{/if}
	</DropdownMenu.Root>
{/if}

{#if !onReport && reportOpen}
	<ReportDialog bind:open={reportOpen} save={item} />
{/if}

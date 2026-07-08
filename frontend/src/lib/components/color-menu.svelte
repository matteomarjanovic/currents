<script lang="ts">
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { toast } from 'svelte-sonner';
	import { copyText } from '$lib/save-actions';
	import Copy from '@lucide/svelte/icons/copy';
	import Compass from '@lucide/svelte/icons/compass';
	import FolderSearch from '@lucide/svelte/icons/folder-search';

	// A palette swatch that opens a compact menu: the color code with a copy
	// button, then search-by-this-color in explore or in the library.
	let {
		hex,
		class: className,
		onExplore,
		onLibrary
	}: {
		hex: string;
		class?: string;
		onExplore: (hex: string) => void;
		onLibrary: (hex: string) => void;
	} = $props();

	let code = $derived(hex.toUpperCase());

	async function copyHex() {
		if (await copyText(code)) toast.success(`Copied ${code}`);
		else toast.error('Could not copy');
	}
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<button
				{...props}
				type="button"
				class={className}
				style="background-color: {hex}"
				aria-label="Color {code}"
				title={code}
			></button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content class="w-60" align="start">
		<div class="flex items-center gap-2 px-2 py-1.5">
			<span class="size-4 shrink-0 rounded-full border" style="background-color: {hex}"></span>
			<span class="flex-1 font-mono text-xs uppercase">{code}</span>
			<button
				type="button"
				onclick={copyHex}
				class="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
				aria-label="Copy hex code"
			>
				<Copy class="size-3.5" />
			</button>
		</div>
		<DropdownMenu.Separator />
		<DropdownMenu.Item onSelect={() => onExplore(hex)}>
			<Compass />
			Search by this color in explore
		</DropdownMenu.Item>
		<DropdownMenu.Item onSelect={() => onLibrary(hex)}>
			<FolderSearch />
			Search by this color in library
		</DropdownMenu.Item>
	</DropdownMenu.Content>
</DropdownMenu.Root>

<script lang="ts">
	import SaveDetail from '$lib/components/save-detail.svelte';
	import type { SaveView } from '$lib/types';

	let {
		save,
		active,
		closing = false
	}: { save: SaveView; active: boolean; closing?: boolean } = $props();
	let scrollRoot: HTMLDivElement | undefined = $state();
</script>

<div
	bind:this={scrollRoot}
	data-save-detail-overlay
	data-active={active}
	inert={!active}
	aria-hidden={active ? undefined : 'true'}
	class="fixed inset-0 z-50 app-muted-wash [overflow-anchor:none] {closing
		? 'overflow-hidden'
		: 'overflow-y-auto'}"
>
	<!-- Mount only once scrollRoot is bound; window scroll belongs to the feed. -->
	{#if scrollRoot}
		<SaveDetail {save} {scrollRoot} {active} />
	{/if}
</div>

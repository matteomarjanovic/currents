<script lang="ts">
	import SaveDetail from '$lib/components/save-detail.svelte';
	import type { SaveView } from '$lib/types';

	let { save, active }: { save: SaveView; active: boolean } = $props();
	let scrollRoot: HTMLDivElement | undefined = $state();
</script>

<div
	bind:this={scrollRoot}
	data-save-detail-overlay
	data-active={active}
	inert={!active}
	aria-hidden={active ? undefined : 'true'}
	class="fixed inset-0 z-50 overflow-y-auto app-muted-wash [overflow-anchor:none]"
>
	<SaveDetail {save} {scrollRoot} />
</div>

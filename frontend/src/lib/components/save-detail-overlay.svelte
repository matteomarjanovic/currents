<script lang="ts">
	import { tick } from 'svelte';
	import SaveDetail from '$lib/components/save-detail.svelte';
	import type { SaveView } from '$lib/types';

	let {
		save,
		active,
		closing = false,
		rendered = true
	}: { save: SaveView; active: boolean; closing?: boolean; rendered?: boolean } = $props();
	let scrollRoot: HTMLDivElement | undefined = $state();
	let content: HTMLDivElement | undefined = $state();
	let savedHeight = $state(0);
	let savedScroll = 0;
	let suspended = false;
	let restoring = $state(false);

	$effect.pre(() => {
		if (!rendered && scrollRoot) {
			savedHeight = scrollRoot.scrollHeight;
			savedScroll = scrollRoot.scrollTop;
			suspended = true;
		}
	});
	$effect(() => {
		if (!rendered || !content || !scrollRoot || !suspended) return;
		const root = scrollRoot;
		const target = content;
		const top = savedScroll;
		restoring = true;
		let frame = 0;
		let cancelled = false;
		// Masonry measures its new nodes in ResizeObserver/rAF. Restore after that layout,
		// while the placeholder keeps the old scroll range and prevents a jump to zero.
		const restore = () => {
			cancelAnimationFrame(frame);
			frame = requestAnimationFrame(() => {
				frame = requestAnimationFrame(() => {
					root.scrollTop = top;
					restoring = false;
					suspended = false;
					observer.disconnect();
				});
			});
		};
		const observer = new ResizeObserver(restore);
		observer.observe(target);
		void tick().then(() => {
			if (!cancelled) restore();
		});
		return () => {
			cancelled = true;
			observer.disconnect();
			cancelAnimationFrame(frame);
		};
	});
</script>

<div
	bind:this={scrollRoot}
	data-save-detail-overlay
	data-active={active}
	data-rendered={rendered}
	inert={!active}
	aria-hidden={active ? undefined : 'true'}
	class="fixed inset-0 z-50 app-muted-wash [overflow-anchor:none] {closing
		? 'overflow-hidden'
		: 'overflow-y-auto'}"
>
	<div style:min-height={!rendered || restoring ? `${savedHeight}px` : undefined}>
		<div bind:this={content} style:visibility={restoring ? 'hidden' : undefined}>
			{#if scrollRoot}
				<SaveDetail {save} {scrollRoot} active={active && !restoring} {rendered} />
			{/if}
		</div>
	</div>
</div>

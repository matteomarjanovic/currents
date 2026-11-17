<script lang="ts">
	import { page } from '$app/state';
	import { prefersReducedMotion } from 'svelte/motion';
	import ModeTabs from '$lib/components/mode-tabs.svelte';
	import ExploreTrail from '$lib/components/explore-trail.svelte';
	import { detailNavigation } from '$lib/save-detail-navigation.svelte';

	let stack = $derived(page.state.saveStack ?? (page.state.save ? [page.state.save] : []));
	let current = $derived(stack.at(-1));
	$effect(() => {
		if (!current) {
			detailNavigation.uri = '';
			detailNavigation.scrollRoot = undefined;
		}
	});
</script>

{#if current}
	<div data-save-detail-navigation class="relative z-50">
		<ModeTabs detail>
			{#snippet trail()}
				<ExploreTrail
					currentHidden={detailNavigation.uri === current.uri &&
						detailNavigation.imageAboveViewport}
					onCurrent={() =>
						detailNavigation.scrollRoot?.scrollTo({
							top: 0,
							behavior: prefersReducedMotion.current ? 'instant' : 'smooth'
						})}
				/>
			{/snippet}
		</ModeTabs>
	</div>
{/if}

<script lang="ts">
	import { page } from '$app/state';
	import { prefersReducedMotion } from 'svelte/motion';
	import ModeTabs from '$lib/components/mode-tabs.svelte';
	import ExploreTrail from '$lib/components/explore-trail.svelte';
	import { IsMobile } from '$lib/hooks/is-mobile.svelte';
	import { detailNavigation } from '$lib/save-detail-navigation.svelte';

	const mobile = new IsMobile();
	let stack = $derived(page.state.saveStack ?? (page.state.save ? [page.state.save] : []));
	let current = $derived(stack.at(-1));
	let currentHidden = $derived(
		detailNavigation.uri === current?.uri && detailNavigation.imageAboveViewport
	);
	$effect(() => {
		if (!current) {
			detailNavigation.uri = '';
			detailNavigation.scrollRoot = undefined;
		}
	});

	function scrollToCurrent() {
		detailNavigation.scrollRoot?.scrollTo({
			top: 0,
			behavior: prefersReducedMotion.current ? 'instant' : 'smooth'
		});
	}
</script>

{#if current && !mobile.current && !page.url.pathname.startsWith('/organize')}
	<div data-save-detail-navigation class="relative z-50">
		<ModeTabs detail>
			{#snippet trail()}
				<ExploreTrail {currentHidden} onCurrent={scrollToCurrent} />
			{/snippet}
		</ModeTabs>
	</div>
{/if}

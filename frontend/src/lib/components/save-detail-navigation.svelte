<script lang="ts">
	import { page } from '$app/state';
	import { prefersReducedMotion } from 'svelte/motion';
	import ModeTabs from '$lib/components/mode-tabs.svelte';
	import ExploreTrail from '$lib/components/explore-trail.svelte';
	import MobileRelatedHistory from '$lib/components/organize/mobile-related-history.svelte';
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

	function revisit(index: number) {
		const delta = index - (stack.length - 1);
		if (delta) history.go(delta);
		else scrollToCurrent();
	}
</script>

{#if current && !page.url.pathname.startsWith('/organize')}
	<div data-save-detail-navigation class="relative z-50">
		{#if mobile.current}
			{#if stack.length > 1 || currentHidden}
				<MobileRelatedHistory
					saves={stack}
					currentIndex={stack.length - 1}
					onRevisit={revisit}
					aboveScrollToTop
				/>
			{/if}
		{:else}
			<ModeTabs detail>
				{#snippet trail()}
					<ExploreTrail {currentHidden} onCurrent={scrollToCurrent} />
				{/snippet}
			</ModeTabs>
		{/if}
	</div>
{/if}

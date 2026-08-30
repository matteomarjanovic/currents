<script lang="ts">
	import { Haptics, ImpactStyle } from '@capacitor/haptics';
	import { isNative } from '$lib/platform';
	import { Spinner } from '$lib/components/ui/spinner';

	interface Props {
		onRefresh: () => Promise<void>;
		label: string;
		disabled?: boolean;
	}

	let { onRefresh, label, disabled = false }: Props = $props();

	const PULL_REFRESH_DISTANCE = 50;
	const nativePullToRefresh = isNative();
	let pullStartY = 0;
	let pullDistance = $state(0);
	let pulling = $state(false);
	let refreshing = $state(false);
	let pulsedRefreshThreshold = false;

	function onPullStart(e: TouchEvent) {
		if (
			!nativePullToRefresh ||
			disabled ||
			refreshing ||
			window.scrollY !== 0 ||
			e.touches.length !== 1 ||
			(e.target instanceof Element &&
				e.target.closest('[data-save-detail-overlay], [role="dialog"]'))
		)
			return;
		pullStartY = e.touches[0].clientY;
		pulsedRefreshThreshold = false;
		pulling = true;
	}

	function onPullMove(e: TouchEvent) {
		if (!pulling || e.touches.length !== 1) return;
		const distance = e.touches[0].clientY - pullStartY;
		pullDistance = distance > 0 ? Math.min(PULL_REFRESH_DISTANCE, distance) : 0;
		if (pullDistance === PULL_REFRESH_DISTANCE && !pulsedRefreshThreshold) {
			pulsedRefreshThreshold = true;
			void Haptics.impact({ style: ImpactStyle.Light });
		}
	}

	async function onPullEnd() {
		if (!pulling) return;
		const shouldRefresh = pullDistance === PULL_REFRESH_DISTANCE;
		pulling = false;
		pullDistance = 0;
		if (!shouldRefresh) return;

		refreshing = true;
		try {
			await onRefresh();
		} finally {
			refreshing = false;
		}
	}
</script>

<svelte:window
	ontouchstart={onPullStart}
	ontouchmove={onPullMove}
	ontouchend={onPullEnd}
	ontouchcancel={onPullEnd}
/>

{#if nativePullToRefresh && (pullDistance > 0 || refreshing)}
	<div
		role="status"
		aria-label={refreshing
			? `Refreshing ${label}`
			: pullDistance === PULL_REFRESH_DISTANCE
				? `Release to refresh ${label}`
				: `Pull to refresh ${label}`}
		class="pointer-events-none fixed left-1/2 z-20 -translate-x-1/2 rounded-full bg-primary-foreground/80 p-2 text-foreground shadow-sm backdrop-blur-sm"
		style="top: calc(env(safe-area-inset-top) + 3.5rem + {refreshing ? 24 : pullDistance}px)"
	>
		{#if refreshing || pullDistance === PULL_REFRESH_DISTANCE}
			<Spinner />
		{:else}
			<span class="block size-4 rounded-full border-2 border-current"></span>
		{/if}
	</div>
{/if}

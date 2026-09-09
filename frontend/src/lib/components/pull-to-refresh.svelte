<script lang="ts">
	import { Haptics, ImpactStyle } from '@capacitor/haptics';
	import { onMount } from 'svelte';
	import { isNative } from '$lib/platform';
	import { Spinner } from '$lib/components/ui/spinner';

	interface Props {
		onRefresh: () => Promise<void>;
		label: string;
		disabled?: boolean;
	}

	let { onRefresh, label, disabled = false }: Props = $props();

	const PULL_REFRESH_DISTANCE = 50;
	const PULL_TRIGGER_DRAG_DISTANCE = 125;
	const nativePullToRefresh = isNative();
	let pullStartY = 0;
	let pullDistance = $state(0);
	let pulling = $state(false);
	let refreshing = $state(false);
	let pulsedRefreshThreshold = false;

	function hasOpenOverlay() {
		return Boolean(
			document.querySelector(
				'[data-save-detail-overlay], [data-slot="dialog-content"], [data-slot="drawer-content"], [data-slot="dropdown-menu-content"], [data-slot="context-menu-content"]'
			)
		);
	}

	onMount(() => {
		window.addEventListener('touchmove', onPullMove, { passive: false });
		return () => window.removeEventListener('touchmove', onPullMove);
	});

	function onPullStart(e: TouchEvent) {
		if (
			!nativePullToRefresh ||
			disabled ||
			refreshing ||
			window.scrollY !== 0 ||
			e.touches.length !== 1 ||
			hasOpenOverlay() ||
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
		if (distance <= 0) {
			pulling = false;
			pullDistance = 0;
			return;
		}
		e.preventDefault();
		const progress = Math.min(1, distance / PULL_TRIGGER_DRAG_DISTANCE);
		pullDistance = PULL_REFRESH_DISTANCE * (1 - (1 - progress) ** 2);
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

<svelte:window ontouchstart={onPullStart} ontouchend={onPullEnd} ontouchcancel={onPullEnd} />

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

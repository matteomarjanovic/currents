<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import FlowField from './flow-field.svelte';

	const OPTIONS = [
		{ slug: 'personal', label: '— Personal', noiseIntensity: 0.5 },
		{ slug: 'balanced', label: '~ Balanced', noiseIntensity: 2 },
		{ slug: 'general', label: '≈ General', noiseIntensity: 4 },
		{ slug: 'new-worlds', label: '✵ New worlds', noiseIntensity: 7 }
	] as const;
	const DEFAULT_INDEX = 1;
	const itemHeight = 40;

	const selectedIndex = $derived.by(() => {
		const slug = page.url.searchParams.get('personalization');
		const idx = OPTIONS.findIndex((o) => o.slug === slug);
		return idx === -1 ? DEFAULT_INDEX : idx;
	});

	let activeIndex = $state(DEFAULT_INDEX);
	let isVisible = $state(false);
	let containerRef = $state<HTMLDivElement | null>(null);
	let startIndex = $state(DEFAULT_INDEX);

	let hoverOpenTimeout: ReturnType<typeof setTimeout> | null = null;
	let hoverCloseTimeout: ReturnType<typeof setTimeout> | null = null;
	let lastTouchCloseTime = 0;
	const isSyntheticAfterTouch = () => Date.now() - lastTouchCloseTime < 500;

	const currentNoiseIntensity = $derived(
		isVisible ? OPTIONS[activeIndex].noiseIntensity : OPTIONS[selectedIndex].noiseIntensity
	);

	function openMenu() {
		if (hoverCloseTimeout) {
			clearTimeout(hoverCloseTimeout);
			hoverCloseTimeout = null;
		}
		activeIndex = selectedIndex;
		isVisible = true;
	}

	function closeMenu() {
		isVisible = false;
		activeIndex = selectedIndex;
	}

	function selectItem(i: number) {
		const url = new URL(page.url);
		url.searchParams.set('personalization', OPTIONS[i].slug);
		goto(url, { replaceState: true, keepFocus: true, noScroll: true });
		isVisible = false;
	}

	function handleButtonMouseEnter() {
		if (isSyntheticAfterTouch()) return;
		if (hoverCloseTimeout) {
			clearTimeout(hoverCloseTimeout);
			hoverCloseTimeout = null;
		}
		if (!isVisible) {
			hoverOpenTimeout = setTimeout(openMenu, 300);
		}
	}

	function handleButtonMouseLeave() {
		if (hoverOpenTimeout) {
			clearTimeout(hoverOpenTimeout);
			hoverOpenTimeout = null;
		}
	}

	function handleButtonClick() {
		if (isSyntheticAfterTouch()) return;
		if (hoverOpenTimeout) {
			clearTimeout(hoverOpenTimeout);
			hoverOpenTimeout = null;
		}
		if (isVisible) closeMenu();
		else openMenu();
	}

	function handleContainerMouseEnter() {
		if (hoverCloseTimeout) {
			clearTimeout(hoverCloseTimeout);
			hoverCloseTimeout = null;
		}
	}

	function handleContainerMouseLeave() {
		if (hoverOpenTimeout) {
			clearTimeout(hoverOpenTimeout);
			hoverOpenTimeout = null;
		}
		if (isVisible) {
			hoverCloseTimeout = setTimeout(closeMenu, 150);
		}
	}

	function updateActiveIndex(clientY: number) {
		if (!containerRef) return;
		const rect = containerRef.getBoundingClientRect();
		const relativeY = clientY - rect.top;
		const index = Math.floor(relativeY / itemHeight);
		activeIndex = Math.max(0, Math.min(OPTIONS.length - 1, index));
	}

	function handleTouchStart(e: TouchEvent) {
		e.preventDefault();
		if (isVisible) {
			isVisible = false;
			lastTouchCloseTime = Date.now();
			return;
		}
		const touch = e.touches[0];
		isVisible = true;
		activeIndex = selectedIndex;
		startIndex = selectedIndex;
		updateActiveIndex(touch.clientY);

		window.addEventListener('touchmove', handleTouchMove, { passive: false });
		window.addEventListener('touchend', handleTouchEnd);
	}

	function handleTouchMove(e: TouchEvent) {
		e.preventDefault();
		if (!isVisible || !containerRef) return;
		const touch = e.touches[0];

		const rect = containerRef.getBoundingClientRect();
		const inside =
			touch.clientX >= rect.left &&
			touch.clientX <= rect.right &&
			touch.clientY >= rect.top &&
			touch.clientY <= rect.bottom;

		if (inside) {
			updateActiveIndex(touch.clientY);
		} else {
			activeIndex = startIndex;
		}
	}

	function handleTouchEnd() {
		if (activeIndex !== selectedIndex) {
			selectItem(activeIndex);
		} else {
			isVisible = false;
		}
		lastTouchCloseTime = Date.now();

		window.removeEventListener('touchmove', handleTouchMove);
		window.removeEventListener('touchend', handleTouchEnd);
	}
</script>

<div
	role="group"
	aria-label="Personalization selector"
	class="relative inline-flex flex-col items-end select-none"
	onmouseenter={handleContainerMouseEnter}
	onmouseleave={handleContainerMouseLeave}
>
	{#if isVisible}
		<div
			bind:this={containerRef}
			class="absolute right-0 bottom-[110%] flex w-48 flex-col overflow-hidden rounded-3xl bg-popover/70 p-1.5 shadow-lg ring-1 ring-foreground/5 backdrop-blur-2xl backdrop-saturate-150 dark:ring-foreground/10"
		>
			{#each OPTIONS as option, i (option.slug)}
				<div
					class="flex h-10 w-full cursor-pointer items-center justify-center rounded-2xl px-3 py-2 text-sm font-medium transition-colors {i ===
					activeIndex
						? 'bg-foreground/10 text-foreground'
						: 'text-popover-foreground hover:bg-foreground/10 hover:text-foreground'}"
					onmouseenter={() => (activeIndex = i)}
					onclick={() => selectItem(i)}
					role="button"
					tabindex="0"
					onkeydown={(e) => e.key === 'Enter' && selectItem(i)}
				>
					{option.label}
				</div>
			{/each}
		</div>
	{/if}

	<button
		onmouseenter={handleButtonMouseEnter}
		onmouseleave={handleButtonMouseLeave}
		onclick={handleButtonClick}
		ontouchstart={handleTouchStart}
		class="z-20 flex h-15 w-15 cursor-pointer items-center justify-center overflow-hidden rounded-full border-none bg-primary-foreground/80 p-0 backdrop-blur-sm transition-transform duration-100 {isVisible
			? 'scale-95'
			: ''}"
		aria-label="Adjust personalization"
		aria-expanded={isVisible}
		aria-haspopup="true"
	>
		<FlowField noiseIntensity={currentNoiseIntensity} />
	</button>
</div>


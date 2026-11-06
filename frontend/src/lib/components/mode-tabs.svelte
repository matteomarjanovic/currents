<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { prefersReducedMotion } from 'svelte/motion';
	import { MotionDiv } from '@humanspeak/svelte-motion';
	import * as Tabs from '$lib/components/ui/animated-tabs';
	import LogoIcon from '$lib/assets/logo-icon.svelte';
	import { modeTabsState } from '$lib/mode-tabs.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import {
		features,
		isFeatureSeen,
		markFeatureSeen,
		FEATURE_ORGANIZE_MODE
	} from '$lib/stores/features.svelte';

	type AppMode = 'explore' | 'organize';

	let { detail = false, viewTransitionName }: { detail?: boolean; viewTransitionName?: string } =
		$props();

	let value = $state<AppMode>('explore');

	let routeMode = $derived<AppMode>(
		page.url.pathname.startsWith('/organize') ? 'organize' : 'explore'
	);
	let supportedRoute = $derived(
		page.url.pathname.startsWith('/organize') ||
			(page.route.id?.startsWith('/(with-navbar)') && page.route.id !== '/(with-navbar)')
	);
	let detailOpen = $derived(!!page.state.save || (page.state.saveStack?.length ?? 0) > 0);
	let visible = $derived(
		detail ||
			(supportedRoute &&
				!detailOpen &&
				(routeMode !== 'organize' || modeTabsState.organizeSidebarOpen))
	);
	let showOrganizeNew = $derived(features.loaded && !isFeatureSeen(FEATURE_ORGANIZE_MODE));

	async function switchMode(next: string) {
		if (next !== 'explore' && next !== 'organize') return;
		value = next;
		if (next === 'organize') void markFeatureSeen(FEATURE_ORGANIZE_MODE);
		if (next === routeMode) return;
		await goto(next === 'organize' ? resolve('organize') : resolve('explore'));
	}

	function openExplore() {
		if (routeMode === 'explore') void goto(resolve('explore'));
	}

	$effect(() => {
		value = routeMode;
		if (routeMode === 'organize' && auth.user) void markFeatureSeen(FEATURE_ORGANIZE_MODE);
	});
</script>

<div
	aria-hidden={!visible}
	inert={!visible}
	class="fixed left-4 z-30 hidden items-center gap-2 transition-opacity duration-150 md:flex {visible
		? 'opacity-100'
		: 'pointer-events-none opacity-0'}"
	style="top: calc(env(safe-area-inset-top) + 0.75rem)"
	data-mode-tabs
	style:--mode-transition-name={viewTransitionName}
>
	<a
		href={routeMode === 'organize' ? resolve('organize') : resolve('')}
		class="flex size-[46px] shrink-0 items-center justify-center rounded-full border border-border bg-primary-foreground/80 bg-clip-padding text-foreground backdrop-blur-sm"
		aria-label="Currents"
	>
		<span class="flex size-[34px] items-center justify-center"><LogoIcon /></span>
	</a>

	{#if auth.user}
		<Tabs.Root {value} onValueChange={switchMode} animated={false} class="gap-0">
			<Tabs.List
				aria-label="Currents mode"
				class="relative h-[35px]! w-[170px] rounded-full border border-border bg-primary-foreground/80 bg-clip-padding backdrop-blur-sm"
			>
				<MotionDiv
					data-mode-indicator
					aria-hidden="true"
					initial={false}
					animate={{ x: value === 'organize' ? '100%' : '0%' }}
					transition={prefersReducedMotion.current
						? { duration: 0 }
						: { type: 'spring', stiffness: 500, damping: 30 }}
					class="pointer-events-none absolute inset-y-[3px] left-[3px] w-[calc((100%-6px)/2)] rounded-full border border-border bg-background dark:border-input dark:bg-input/30"
				/>
				<Tabs.Trigger
					value="explore"
					class="h-full rounded-full data-[state=active]:border-transparent! data-[state=active]:bg-transparent! data-[state=active]:shadow-none!"
					onclick={openExplore}>Explore</Tabs.Trigger
				>
				<Tabs.Trigger
					value="organize"
					class="h-full rounded-full data-[state=active]:border-transparent! data-[state=active]:bg-transparent! data-[state=active]:shadow-none!"
				>
					<span class="relative">
						Organize
						{#if showOrganizeNew && routeMode === 'explore'}
							<span
								class="absolute -top-1.5 -right-2 size-2 rounded-full bg-red-500"
								aria-label="New feature available"
							></span>
						{/if}
					</span>
				</Tabs.Trigger>
			</Tabs.List>
		</Tabs.Root>
	{/if}
</div>

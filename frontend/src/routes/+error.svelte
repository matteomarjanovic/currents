<script lang="ts">
	import './layout.css';
	import favicon from '$lib/assets/favicon.svg';
	import { ModeWatcher } from 'mode-watcher';
	import { page } from '$app/state';
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import Logo from '$lib/assets/logo.svelte';
	import ArrowLeft from '@lucide/svelte/icons/arrow-left';

	const isNotFound = $derived(page.status === 404);
	const heading = $derived(isNotFound ? 'This page drifted away.' : 'Something went off course.');
	const detail = $derived(
		isNotFound
			? "The page you're looking for doesn't exist or has moved."
			: (page.error?.message ?? 'An unexpected error occurred.')
	);
</script>

<ModeWatcher />
<svelte:head>
	<link rel="icon" href={favicon} />
	<title>{page.status} · Currents</title>
</svelte:head>

<main class="flex min-h-screen flex-col items-center justify-center px-6 text-center">
	<a href={resolve('/')} class="mb-12 h-5 text-foreground" aria-label="Currents home">
		<Logo />
	</a>

	<p
		class="text-8xl font-semibold tracking-tight text-muted-foreground/40 md:text-9xl"
	>
		{page.status}
	</p>

	<h1 class="mt-6 text-2xl font-semibold tracking-tight text-foreground md:text-3xl">
		{heading}
	</h1>

	<p class="mt-3 max-w-md text-balance text-muted-foreground">
		{detail}
	</p>

	<a href={resolve('/explore')} class="mt-8">
		<Button size="lg" class="gap-2 rounded-full px-8">
			<ArrowLeft class="size-4" />
			Back to explore
		</Button>
	</a>
</main>

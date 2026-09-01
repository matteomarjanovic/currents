<script lang="ts">
	import '../layout.css';
	import { resolve } from '$app/paths';
	import { ModeWatcher } from 'mode-watcher';
	import LogoMerged from '$lib/assets/logo.svelte';
	import SiteFooter from '$lib/components/site-footer.svelte';
	import ThemeToggle from '$lib/components/theme-toggle.svelte';
	import { STANDARD_SITE_DID, STANDARD_SITE_PUBLICATION_RKEY } from '$lib/standard-site';

	let { children } = $props();
</script>

<svelte:head>
	{#if STANDARD_SITE_PUBLICATION_RKEY}
		<link
			rel="site.standard.publication"
			href="at://{STANDARD_SITE_DID}/site.standard.publication/{STANDARD_SITE_PUBLICATION_RKEY}"
		/>
	{/if}
</svelte:head>

<ModeWatcher />

<div class="min-h-svh px-4 py-8 md:px-8">
	<div class="mx-auto flex max-w-2xl flex-col gap-8">
		<header class="flex items-center justify-between">
			<a href={resolve('/')} class="flex h-5 items-center gap-2 font-medium">
				<LogoMerged />
			</a>
			<div class="flex items-center gap-3">
				<a
					href={resolve('/blog')}
					class="text-sm text-muted-foreground underline underline-offset-4">Blog</a
				>
				<a href={resolve('/')} class="text-sm text-muted-foreground underline underline-offset-4"
					>Go to app</a
				>
				<ThemeToggle />
			</div>
		</header>

		<main>
			{@render children()}
		</main>
	</div>
</div>

<SiteFooter class="mt-10" />

<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const dateFormat = new Intl.DateTimeFormat('en-US', { dateStyle: 'long' });
</script>

<svelte:head>
	<title>Into the currents · Currents</title>
	<meta name="description" content="News and notes from the Currents team." />
</svelte:head>

<div class="flex flex-col gap-4">
	<div class="relative overflow-hidden rounded-2xl">
		<img src="/blog/banner.webp" alt="" width="1400" height="547" class="w-full" />
		<div
			class="absolute inset-0 bg-linear-to-t from-black/90 via-black/50 via-70% to-transparent to-85%"
		></div>
		<div class="absolute inset-x-0 bottom-0 flex flex-col gap-2 p-6">
			<h1 class="text-3xl leading-tight font-semibold text-white">Into the currents</h1>
			<p class="max-w-md text-sm text-white/80">
				A blog where we publish updates about the development of the app and insights about the
				direction we're following.
			</p>
		</div>
	</div>

	<h2 class="mt-4 text-2xl font-semibold text-foreground">Articles</h2>

	<ul class="flex flex-col gap-6">
		{#each data.posts as post (post.slug)}
			<li>
				<a href="/blog/{post.slug}" class="group flex flex-col gap-1">
					<span class="text-xl font-medium text-foreground group-hover:underline">
						{post.title}
					</span>
					<span class="text-sm text-muted-foreground">{dateFormat.format(new Date(post.date))}</span
					>
					<p class="text-sm text-foreground/90">{post.description}</p>
				</a>
			</li>
		{/each}
	</ul>
</div>

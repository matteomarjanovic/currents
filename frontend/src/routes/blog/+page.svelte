<script lang="ts">
	import type { PageData } from './$types';

	let { data }: { data: PageData } = $props();

	const dateFormat = new Intl.DateTimeFormat('en-US', { dateStyle: 'long' });
</script>

<svelte:head>
	<title>Into the currents · Currents</title>
	<meta name="description" content="News and notes from the Currents team." />
</svelte:head>

<div class="flex flex-col gap-6">
	<h1 class="text-3xl leading-tight font-semibold text-foreground">Into the currents</h1>
	<p class="text-md pb-4 text-foreground/90">
		<i>Into the currents</i> is our blog where we publish updates about the development of the app and
		insights about the direction we're following.
	</p>

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

<script lang="ts">
	import { onMount } from 'svelte';
	import { Button } from '$lib/components/ui/button';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import { resolve } from '$app/paths';
	import SiteFooter from '$lib/components/site-footer.svelte';

	let video: HTMLVideoElement;

	onMount(() => {
		video.muted = true;
		video.play().catch(() => {});

		const onVisibility = () => {
			if (!document.hidden) video.play().catch(() => {});
		};
		document.addEventListener('visibilitychange', onVisibility);
		return () => document.removeEventListener('visibilitychange', onVisibility);
	});
</script>

<div class="relative text-foreground" style="--landing-top-bar-height: 3.75rem;">
	<section
		class="relative isolate h-screen overflow-hidden px-6 py-8 sm:px-8 sm:py-10 lg:px-10 lg:py-8"
		style="margin-top: calc(-1 * var(--landing-top-bar-height));"
	>
		<video
			bind:this={video}
			class="pointer-events-none absolute inset-0 -z-20 h-full w-full object-cover"
			autoplay
			muted
			loop
			playsinline
			disablepictureinpicture
			aria-hidden="true"
		>
			<source src="/video/currents_hero.mp4" type="video/mp4" />
		</video>

		<div class="absolute inset-0 -z-10 bg-background/55"></div>
		<div
			class="absolute inset-0 -z-10"
			style="background: linear-gradient(180deg, color-mix(in oklch, var(--muted) 50%, transparent) 0%, color-mix(in oklch, var(--background) 72%, transparent) 100%);"
		></div>

		<div class="mx-auto flex h-full w-full max-w-4xl items-center justify-center">
			<div class="relative z-10 flex max-w-3xl flex-col items-center justify-center text-center">
				<h1
					class="text-beauty mb-8 max-w-[10ch] font-sans text-5xl leading-[0.98] font-semibold tracking-tight text-foreground md:text-7xl xl:text-[5.25rem]"
				>
					Get carried by the currents.
				</h1>

				<p
					class="text-beauty mb-10 max-w-2xl text-lg leading-relaxed text-foreground/82 md:text-xl"
				>
					Or create your own. Save inspiration from anywhere, curate collections that reflect your
					taste, and tune your feed to find exactly what you love.
				</p>

				<div class="flex items-center justify-center gap-4">
					<a href={resolve('/explore')}>
						<Button size="lg" class="gap-2 rounded-full px-8 text-base shadow-lg shadow-black/20">
							Explore currents
							<ArrowRight class="size-4" />
						</Button>
					</a>
				</div>
			</div>
		</div>
	</section>

	<SiteFooter />
</div>

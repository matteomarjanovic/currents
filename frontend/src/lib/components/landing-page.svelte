<script lang="ts">
	import LandingFlowV3 from './landing-flow.svelte';
	import { Button } from '$lib/components/ui/button';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import { resolve } from '$app/paths';
	import Logo from '$lib/assets/logo.svelte';

	const githubUrl = 'https://github.com/matteomarjanovic/currents';
	const blueskyUrl = 'https://bsky.app/profile/currents.is';
</script>

<div class="relative text-foreground">
	<!-- Hero: full-width, flow fills the background at all viewport sizes -->
	<section
		class="relative flex min-h-dvh flex-col items-center justify-center overflow-hidden px-8 pt-0 pb-16 text-center"
	>
		<!-- Responsive masonry flow fills the entire hero -->
		<div class="absolute inset-0 z-0">
			<LandingFlowV3 />
		</div>

		<!-- Vignette: centered radial, strong in the middle for readability -->
		<div
			class="pointer-events-none absolute inset-0 z-5"
			style="background: radial-gradient(ellipse 90% 80% at 50% 48%, color-mix(in oklch, var(--background) 96%, transparent) 0%, color-mix(in oklch, var(--background) 86%, transparent) 30%, color-mix(in oklch, var(--background) 38%, transparent) 62%, transparent 86%);"
		></div>

		<!-- Text content -->
		<div class="relative z-9 max-w-xl">
			<h1
				class="text-beauty mb-10 font-sans text-6xl leading-[1.02] font-semibold tracking-tight text-foreground md:text-7xl lg:mt-16 xl:text-[5.25rem]"
			>
				Get carried by the currents.
			</h1>

			<p class="text-beauty mb-12 text-lg leading-relaxed text-muted-foreground md:text-xl">
				Or create your own. Save inspiration from anywhere, curate collections that reflect your
				taste, and tune your feed to find exactly what you love.
			</p>

			<div class="flex items-center justify-center gap-4">
				<a href={resolve('/explore')}>
					<Button size="lg" class="gap-2 rounded-full px-8 text-base">
						Explore currents
						<ArrowRight class="size-4" />
					</Button>
				</a>
				<!-- <a
					href="https://github.com/mmarjanovic/currents"
					target="_blank"
					rel="noopener noreferrer"
					class="text-sm text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
				>
					View source
				</a> -->
			</div>
		</div>
	</section>

	<!-- Footer -->
	<footer class="border-t border-border/60 px-8 py-8">
		<div class="mx-auto flex max-w-5xl flex-col gap-6 sm:flex-row sm:items-end sm:justify-between">
			<div class="space-y-3 text-center sm:text-left">
				<a
					href={resolve('/')}
					class="mx-auto block h-5 w-fit text-lg font-semibold text-foreground sm:mx-0"
				>
					<Logo />
				</a>
				<p class="max-w-md text-sm leading-relaxed text-muted-foreground">
					A calm visual curation app for the open social web.
				</p>
				<p class="text-sm text-muted-foreground">&copy; {new Date().getFullYear()} Currents.</p>
			</div>

			<div class="flex flex-col items-center gap-3 sm:items-end">
				<div class="flex items-center justify-center gap-2 sm:justify-end">
					<Button
						variant="outline"
						size="icon-lg"
						href={githubUrl}
						target="_blank"
						rel="noreferrer"
						aria-label="Currents on GitHub"
						class="rounded-full border-border/80 bg-background/70 backdrop-blur-sm"
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							viewBox="0 0 16 16"
							fill="currentColor"
							class="size-4"
						>
							<path
								d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82A7.65 7.65 0 0 1 8 4.77c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z"
							/>
						</svg>
					</Button>
					<Button
						variant="outline"
						size="icon-lg"
						href={blueskyUrl}
						target="_blank"
						rel="noreferrer"
						aria-label="Currents on Bluesky"
						class="rounded-full border-border/80 bg-background/70 backdrop-blur-sm"
					>
						<svg
							xmlns="http://www.w3.org/2000/svg"
							viewBox="0 0 576 512"
							fill="currentColor"
							class="size-4"
						>
							<path
								d="M407.8 294.7c-3.3-.4-6.7-.8-10-1.3 3.4 .4 6.7 .9 10 1.3zM288 227.1C261.9 176.4 190.9 81.9 124.9 35.3 61.6-9.4 37.5-1.7 21.6 5.5 3.3 13.8 0 41.9 0 58.4S9.1 194 15 213.9c19.5 65.7 89.1 87.9 153.2 80.7 3.3-.5 6.6-.9 10-1.4-3.3 .5-6.6 1-10 1.4-93.9 14-177.3 48.2-67.9 169.9 120.3 124.6 164.8-26.7 187.7-103.4 22.9 76.7 49.2 222.5 185.6 103.4 102.4-103.4 28.1-156-65.8-169.9-3.3-.4-6.7-.8-10-1.3 3.4 .4 6.7 .9 10 1.3 64.1 7.1 133.6-15.1 153.2-80.7 5.9-19.9 15-138.9 15-155.5s-3.3-44.7-21.6-52.9c-15.8-7.1-40-14.9-103.2 29.8-66.1 46.6-137.1 141.1-163.2 191.8z"
							/>
						</svg>
					</Button>
				</div>
				<div
					class="flex flex-wrap items-center justify-center gap-x-4 gap-y-1 text-sm text-muted-foreground sm:justify-end"
				>
					<a
						class="underline-offset-4 transition-colors hover:text-foreground hover:underline"
						href={resolve('/terms')}
					>
						Terms of Service
					</a>
					<a
						class="underline-offset-4 transition-colors hover:text-foreground hover:underline"
						href={resolve('/privacy')}
					>
						Privacy Policy
					</a>
				</div>
			</div>
		</div>
	</footer>
</div>

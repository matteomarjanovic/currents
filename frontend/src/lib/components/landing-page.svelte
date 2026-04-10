<script lang="ts">
	import LandingFlowV3 from './landing-flow.svelte';
	import { Button } from '$lib/components/ui/button';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import FeatureEight from './blocks/feature/feature-eight.svelte';
</script>

<div class="relative bg-background text-foreground">
	<!-- Hero: full-width, flow fills the background at all viewport sizes -->
	<section
		class="relative flex min-h-[100dvh] flex-col items-center justify-center overflow-hidden px-8 pb-16 pt-0 text-center"
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
				class="font-italiana mb-10 leading-[1.02] tracking-tight text-foreground text-beauty text-6xl md:text-7xl lg:mt-16 xl:text-[5.25rem]"
				// style="font-size: clamp(3.4rem, 6.5vw, 6.2rem);"
			>
				Get carried by the currents.
			</h1>

			<p class="mb-12 text-lg leading-relaxed text-muted-foreground text-beauty md:text-xl">
				Or create your own. Save inspiration from anywhere, curate collections that reflect your taste, and tune your feed to find exactly what you love.
			</p>

			<div class="flex items-center justify-center gap-4">
				<a href="/explore">
					<Button
						size="lg"
						class="gap-2 rounded-full px-8 text-base"
					>
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

	<!-- <FeatureEight />

	<section class="px-8 pb-8 md:px-14 lg:px-20 xl:px-28">
		<div class="space-y-28 xl:space-y-36">
			<div class="max-w-lg">
				<span class="font-italiana select-none text-6xl text-border">01</span>
				<h3 class="font-italiana mt-4 text-4xl text-foreground lg:text-5xl">Save everything.</h3>
				<p class="mt-5 max-w-[44ch] leading-relaxed text-muted-foreground">
					The browser extension clips images from any site in one click. Organize them into
					collections that grow and evolve with you.
				</p>
			</div>

			<div class="max-w-lg md:ml-auto md:text-right">
				<span class="font-italiana select-none text-6xl text-border">02</span>
				<h3 class="font-italiana mt-4 text-4xl text-foreground lg:text-5xl">Discover more.</h3>
				<p class="mt-5 max-w-[44ch] leading-relaxed text-muted-foreground md:ml-auto">
					The feed reads your collections' visual fingerprint and surfaces images you'll actually
					care about. No engagement traps.
				</p>
			</div>

			<div class="max-w-lg">
				<span class="font-italiana select-none text-6xl text-border">03</span>
				<h3 class="font-italiana mt-4 text-4xl text-foreground lg:text-5xl">Flow, not scroll.</h3>
				<p class="mt-5 max-w-[44ch] leading-relaxed text-muted-foreground">
					Built on AT Protocol. Your data is yours — portable across the open social web, always
					under your control.
				</p>
			</div>
		</div>


		<div class="mt-36 border-t border-border pb-28 pt-20">
			<p class="mb-4 text-xs font-medium uppercase tracking-[0.2em] text-muted-foreground">
				Ready?
			</p>
			<h2
				class="font-italiana mb-10 leading-tight text-foreground"
				style="font-size: clamp(2.8rem, 5vw, 5rem);"
			>
				Start collecting.
			</h2>
			<a href="/login">
				<Button
					variant="outline"
					size="lg"
					class="gap-2 rounded-full px-8 text-base transition-transform duration-200 active:scale-[0.98]"
				>
					Sign in with Bluesky
					<ArrowRight class="size-4" />
				</Button>
			</a>
		</div>
	</section> -->
	<!-- Footer -->
	<footer class="px-8 py-6 text-center text-sm text-muted-foreground">
		&copy; {new Date().getFullYear()} Currents. Built by <a href="https://matteomarjanovic.com" class="underline hover:text-foreground">Matteo Marjanovic</a>.
	</footer>
</div>

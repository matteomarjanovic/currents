<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import Logo from '$lib/assets/logo.svelte';
</script>

<div
	class="relative flex min-h-svh flex-col items-center justify-between gap-12 overflow-hidden px-6 pb-10 pt-[env(safe-area-inset-top,1.5rem)]"
>
	<video
		class="absolute inset-0 -z-20 h-full w-full object-cover"
		autoplay
		muted
		loop
		playsinline
		aria-hidden="true"
	>
		<source src="/video/currents_hero.mp4" type="video/mp4" />
	</video>
	<div class="absolute inset-0 -z-10 bg-background/55"></div>
	<div
		class="absolute inset-0 -z-10"
		style="background: linear-gradient(180deg, color-mix(in oklch, var(--muted) 50%, transparent) 0%, color-mix(in oklch, var(--background) 72%, transparent) 100%);"
	></div>

	<div class="flex h-8 w-full items-center justify-center pt-6 text-foreground">
		<Logo />
	</div>

	<div class="flex max-w-md flex-col items-center text-center">
		<h1
			class="text-beauty mb-6 font-sans text-5xl leading-[0.98] font-semibold tracking-tight text-foreground"
		>
			Get carried by the currents.
		</h1>
		<p class="text-beauty max-w-sm text-base leading-relaxed text-foreground/82">
			Save inspiration from anywhere, curate collections that reflect your taste, and tune your feed
			to find exactly what you love.
		</p>
	</div>

	<div class="flex w-full max-w-sm flex-col gap-3">
		<a href={resolve('/login')} class="block">
			<Button size="lg" class="w-full rounded-full text-base shadow-lg shadow-black/20">
				Log in
			</Button>
		</a>
		<a href={resolve('/register')} class="block">
			<Button
				variant="outline"
				size="lg"
				class="w-full rounded-full bg-background/40 text-base backdrop-blur-sm"
			>
				Sign up
			</Button>
		</a>
	</div>
</div>

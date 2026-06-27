<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import Logo from '$lib/assets/logo.svelte';
</script>

<div
	class="flex min-h-svh flex-col items-center justify-between gap-12 bg-secondary pt-[calc(env(safe-area-inset-top,1.5rem)+1.5rem)] pb-10"
>
	<!-- h-7 ≈ the top-bar logo size; mt (not pt) for the offset so the fixed height isn't eaten. -->
	<div class="mt-10 flex h-7 w-full items-center justify-center text-foreground">
		<Logo />
	</div>

	<div class="flex max-w-md flex-col items-center px-6 text-center">
		<h1
			class="text-beauty mb-6 font-sans text-5xl leading-[0.98] font-semibold tracking-tight text-foreground"
		>
			Welcome to Currents
		</h1>
		<p class="text-beauty max-w-sm text-base leading-relaxed text-foreground/82">
			Save inspiration from anywhere, curate collections that reflect your taste, and tune your feed
			to find exactly what you love.
		</p>
	</div>

	<div class="flex w-full max-w-sm flex-col gap-3 px-6">
		<a href={resolve('/login')} class="block">
			<Button size="lg" class="w-full rounded-full">Log in</Button>
		</a>
		<a href={resolve('/register')} class="block">
			<Button variant="outline" size="lg" class="w-full rounded-full">Sign up</Button>
		</a>
	</div>
</div>

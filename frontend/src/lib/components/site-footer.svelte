<script lang="ts">
	import { resolve } from '$app/paths';
	import { Button } from '$lib/components/ui/button';
	import Logo from '$lib/assets/logo.svelte';
	import { cn } from '$lib/utils.js';
	import { shouldOpenExternally, openExternal } from '$lib/external';

	// The blog is prerendered outside the SPA — from the native app / PWA open it in the
	// system browser rather than navigating in-app (a no-op on regular web).
	function handleBlog(e: MouseEvent) {
		if (!shouldOpenExternally()) return;
		e.preventDefault();
		openExternal('/blog');
	}

	// The site-wide footer (landing page, support page): logo + tagline,
	// social links, and the legal/info links.
	const githubUrl = 'https://github.com/matteomarjanovic/currents';
	const blueskyUrl = 'https://bsky.app/profile/currents.is';

	let { class: className = '' }: { class?: string } = $props();
</script>

<footer class={cn('relative z-10 border-t border-border/60 app-muted-wash px-8 py-8', className)}>
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
			<p class="text-xs text-muted-foreground">Matteo Marjanovic | VAT: IT05179170237</p>
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
							d="M6.766 11.328c-2.063-.25-3.516-1.734-3.516-3.656 0-.781.281-1.625.75-2.188-.203-.515-.172-1.609.063-2.062.625-.078 1.468.25 1.968.703.594-.187 1.219-.281 1.985-.281.765 0 1.39.094 1.953.265.484-.437 1.344-.765 1.969-.687.218.422.25 1.515.046 2.047.5.593.766 1.39.766 2.203 0 1.922-1.453 3.375-3.547 3.64.531.344.89 1.094.89 1.954v1.625c0 .468.391.734.86.547C13.781 14.359 16 11.53 16 8.03 16 3.61 12.406 0 7.984 0 3.563 0 0 3.61 0 8.031a7.88 7.88 0 0 0 5.172 7.422c.422.156.828-.125.828-.547v-1.25c-.219.094-.5.156-.75.156-1.031 0-1.64-.562-2.078-1.609-.172-.422-.36-.672-.719-.719-.187-.015-.25-.093-.25-.187 0-.188.313-.328.625-.328.453 0 .844.281 1.25.86.313.452.64.655 1.031.655s.641-.14 1-.5c.266-.265.47-.5.657-.656"
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
					href={resolve('/blog')}
					onclick={handleBlog}
				>
					Blog
				</a>
				<a
					class="underline-offset-4 transition-colors hover:text-foreground hover:underline"
					href={resolve('/support-us')}
				>
					Support the project
				</a>
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
				<a
					class="underline-offset-4 transition-colors hover:text-foreground hover:underline"
					href={resolve('/refunds')}
				>
					Refund Policy
				</a>
			</div>
		</div>
	</div>
</footer>

<script lang="ts">
	import { asset } from '$app/paths';
	import LogoIcon from '$lib/assets/logo-icon.svelte';
	import { Button } from '$lib/components/ui/button';
	import * as Tooltip from '$lib/components/ui/tooltip';
	import Pause from '@lucide/svelte/icons/pause';
	import Play from '@lucide/svelte/icons/play';

	const apps = [
		{ name: 'Offprint', icon: 'offprint', href: 'https://offprint.app/', x: 14, y: 24, angle: -7 },
		{ name: 'Popfeed', icon: 'popfeed', href: 'https://popfeed.social/', x: 37, y: 16, angle: 5 },
		{ name: 'Bluesky', icon: 'bluesky', href: 'https://bsky.app/', x: 62, y: 20, angle: -4 },
		{ name: 'Tangled', icon: 'tangled', href: 'https://tangled.org/', x: 85, y: 17, angle: 7 },
		{ name: 'Grain', icon: 'grain', href: 'https://grain.social/', x: 25, y: 46, angle: 6 },
		{ name: 'Spark', icon: 'spark', href: 'https://sprk.so/', x: 75, y: 43, angle: -6 },
		{ name: 'Sifa ID', icon: 'sifa', href: 'https://sifa.id/', x: 90, y: 39, angle: 5 },
		{ name: 'mu', icon: 'mu', href: 'https://mu.social/', x: 12, y: 61, angle: -4 },
		{ name: 'Semble', icon: 'semble', href: 'https://semble.so/', x: 86, y: 63, angle: 7 },
		{ name: 'pckt', icon: 'pckt', href: 'https://pckt.blog/', x: 15, y: 85, angle: -8 },
		{ name: 'Margin', icon: 'margin', href: 'https://margin.at/', x: 36, y: 78, angle: 5 },
		{ name: 'Leaflet', icon: 'leaflet', href: 'https://leaflet.pub/', x: 56, y: 85, angle: -5 },
		{ name: 'Surf', icon: 'surf', href: 'https://surf.social/', x: 73, y: 82, angle: 4 }
	] as const;
	let visible = $state(false);
	let paused = $state(false);
	const motionLabel = $derived(paused ? 'Resume logo motion' : 'Pause logo motion');

	function observeVisibility(figure: HTMLElement) {
		const observer = new IntersectionObserver(
			([entry]) => {
				visible = entry.intersectionRatio >= 0.25;
			},
			{ threshold: 0.25 }
		);
		observer.observe(figure);
		return () => observer.disconnect();
	}
</script>

<figure
	class="atmosphere"
	aria-label="Currents and other apps in the Atmosphere"
	data-playing={visible && !paused ? '' : undefined}
	{@attach observeVisibility}
>
	<div class="app-mark currents" role="img" aria-label="Currents" title="Currents">
		<LogoIcon />
	</div>

	<Tooltip.Provider delayDuration={250}>
		{#each apps as app (app.name)}
			<Tooltip.Root>
				<Tooltip.Trigger>
					{#snippet child({ props })}
						<a
							{...props}
							class="app-mark"
							type={undefined}
							href={app.href}
							target="_blank"
							rel="noopener noreferrer"
							aria-label={app.name}
							style:--x={`${app.x}%`}
							style:--y={`${app.y}%`}
							style:--angle={`${app.angle}deg`}
							style:--phase={`${(app.x - 100) * 44}ms`}
						>
							<img
								src={asset(`landing/atmosphere/${app.icon}.webp`)}
								alt=""
								width="128"
								height="128"
								loading="lazy"
								decoding="async"
							/>
						</a>
					{/snippet}
				</Tooltip.Trigger>
				<Tooltip.Content sideOffset={8} arrowClasses="hidden" class="rounded-lg px-2 py-0.5">
					{app.name}
				</Tooltip.Content>
			</Tooltip.Root>
		{/each}
	</Tooltip.Provider>

	<Button
		variant="ghost"
		size="icon-sm"
		class="absolute right-2 bottom-2 text-muted-foreground motion-reduce:hidden"
		aria-label={motionLabel}
		title={motionLabel}
		onclick={() => (paused = !paused)}
	>
		{#if paused}<Play class="size-3.5" />{:else}<Pause class="size-3.5" />{/if}
	</Button>
</figure>

<style>
	.atmosphere {
		position: relative;
		aspect-ratio: 64 / 45;
		border: 1px solid color-mix(in srgb, var(--foreground) 10%, transparent);
		border-radius: var(--radius);
		background: linear-gradient(140deg, #192330, #111923 57%, #18232f);
	}
	.app-mark {
		position: absolute;
		top: var(--y, 50%);
		left: var(--x, 50%);
		width: clamp(1.875rem, 9.5%, 3.75rem);
		aspect-ratio: 1;
		transform: translate(-50%, -50%);
		border-radius: var(--radius);
	}
	.app-mark img {
		display: block;
		width: 100%;
		height: 100%;
		border: 1px solid #c3d9f038;
		border-radius: 29%;
		background: #17202b;
		box-shadow: 0 0.32rem 0.9rem #070b1266;
		rotate: var(--angle);
	}
	a.app-mark:is(:hover, :focus-visible) {
		z-index: 1;
	}
	a.app-mark:focus-visible {
		outline: 2px solid #fff;
		outline-offset: 0.35rem;
	}
	.currents {
		display: grid;
		place-items: center;
		width: clamp(3.5rem, 18%, 7.25rem);
		padding: clamp(0.4rem, 2%, 0.75rem);
		border-radius: 50%;
		color: #0a0a0a;
		background: #fff;
		box-shadow: 0 0.65rem 1.5rem #070b1266;
	}
	@media (prefers-reduced-motion: no-preference) {
		.app-mark img {
			animation: swell 7s ease-in-out var(--phase) infinite paused;
		}
		.atmosphere[data-playing] a.app-mark:not(:hover, :focus-visible) img {
			animation-play-state: running;
		}
	}
	@keyframes swell {
		0%,
		100% {
			transform: translate(-4%, -10%) rotate(-1.5deg);
		}
		50% {
			transform: translate(4%, 10%) rotate(1.5deg);
		}
	}
</style>

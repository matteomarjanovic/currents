<script lang="ts">
	import { asset, resolve } from '$app/paths';
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import { Button } from '$lib/components/ui/button';
	import SiteFooter from '$lib/components/site-footer.svelte';
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import { trackEvent } from '$lib/analytics';
	import Hero from './landing/hero.svelte';
	import Modes from './landing/modes.svelte';
	import Features from './landing/features.svelte';
	import Atmosphere from './landing/atmosphere.svelte';
</script>

{#snippet explore()}
	<Button
		href={resolve('explore')}
		size="lg"
		class="h-12 gap-3 px-6 text-base"
		onclick={() => trackEvent('landing_cta_clicked', { destination: 'explore' })}
	>
		Explore Currents <ArrowRight />
	</Button>
{/snippet}

<div class="landing" data-web-landing>
	<main>
		<Hero action={explore} />

		<section class="wrap introduction split" aria-labelledby="landing-intro">
			<h2 id="landing-intro" class="landing-heading">Inspiration deserves its own space.</h2>
			<div class="prose-copy">
				<p>
					The internet is full of things worth keeping. A photograph you want to come back to, a
					color for your next project, an image you can’t quite explain.
				</p>
				<p>
					Currents gives you a place to collect those things and make connections between them.
					<span class="emphasis">A visual library that grows with your curiosity.</span>
				</p>
			</div>
		</section>

		<div class="mode-section">
			<div class="wrap"><Modes /></div>
		</div>

		<div class="wrap feature-section"><Features /></div>

		<section class="philosophy" aria-labelledby="landing-philosophy">
			<div class="wrap">
				<h2 id="landing-philosophy" class="landing-heading">A quiet place to explore.</h2>
				<p>
					Start with a color you keep noticing. Follow it through photographs, paintings, and places
					you’ve never seen. Supporters keep Currents ad-free, so we can build it around what you
					came here to see.
				</p>
			</div>
		</section>

		<section class="wrap split atmosphere-section" aria-labelledby="landing-atmosphere">
			<div>
				<p class="section-name">Part of the Atmosphere</p>
				<h2 id="landing-atmosphere" class="landing-heading">A new generation of the social web.</h2>
				<div class="prose-copy">
					<p>
						Collect references in Currents, publish a studio journal in Offprint, or share a photo
						series in Grain. One account works across all three. If you’re on Bluesky, you already
						have one.
					</p>
					<p>
						Your visual library stays with your account. If you ever change account providers, you
						can take your public saves and collections with you.
					</p>
				</div>
				<a href="https://atmosphereaccount.com/" class="text-link">
					Explore the Atmosphere <ArrowUpRight class="size-4" />
				</a>
			</div>
			<Atmosphere />
		</section>

		<section class="support-section" aria-labelledby="landing-support">
			<div class="wrap split">
				<h2 id="landing-support" class="landing-heading">
					An independent project, supported by you.
				</h2>
				<div class="prose-copy">
					<p>
						Currents is open source and built for people who want a calmer, more creative internet.
					</p>
					<p>
						Supporters keep Currents ad-free and in active development, and unlock more ways to
						explore their visual library.
					</p>
					<div class="relative mt-2 w-fit">
						<Button
							href={resolve('support-us')}
							variant="secondary"
							size="lg"
							class="h-12 gap-2.5 border-border pr-5 pl-12 text-base"
						>
							Support the project <ArrowRight />
						</Button>
						<span
							class="pointer-events-none absolute top-1/2 left-5 flex -translate-y-1/2"
							aria-hidden="true"
						>
							<SupporterBadge class="size-5" />
						</span>
					</div>
				</div>
			</div>
		</section>

		<section class="invitation" aria-labelledby="landing-invitation">
			<img
				src={asset('landing/closing-tide.webp')}
				srcset={`${asset('landing/closing-tide-small.webp')} 720w, ${asset('landing/closing-tide.webp')} 1440w`}
				sizes="100vw"
				width="1440"
				height="960"
				alt=""
				loading="lazy"
				decoding="async"
			/>
			<div class="wrap invitation-copy">
				<h2 id="landing-invitation" class="landing-heading">See where your curiosity takes you.</h2>
				<p>There’s always something new to discover.</p>
				{@render explore()}
			</div>
		</section>
	</main>

	<SiteFooter landing />
</div>

<style>
	.landing {
		--landing-paper: color-mix(in srgb, var(--background) 86%, var(--muted));
		--landing-ink: color-mix(in srgb, var(--foreground) 73%, var(--background));
		--landing-gutter: clamp(1.25rem, 5.75vw, 6.5rem);
		background: var(--background);
	}

	.wrap {
		width: min(100% - var(--landing-gutter) * 2, 76rem);
		margin-inline: auto;
	}

	.landing :global(.landing-heading) {
		font-size: clamp(2.7rem, 5.3vw, 5.3rem);
		font-weight: 560;
		line-height: 0.98;
		letter-spacing: -0.054em;
		text-wrap: balance;
	}

	.split {
		display: grid;
		grid-template-columns: minmax(0, 1.05fr) minmax(0, 0.95fr);
		align-items: start;
		gap: 2rem clamp(1.75rem, 8vw, 9rem);
	}
	.split h2 {
		text-wrap: pretty;
	}

	.introduction {
		align-items: center;
		padding-block: clamp(4.5rem, 10vw, 9rem);
	}
	.introduction h2 {
		max-width: 11ch;
	}

	.prose-copy {
		display: grid;
		gap: 0.85em;
		color: var(--landing-ink);
		font-size: clamp(1.05rem, 1.5vw, 1.28rem);
		line-height: 1.52;
	}
	.emphasis {
		color: var(--foreground);
	}

	.mode-section {
		border-block: 1px solid var(--border);
		background: var(--landing-paper);
		padding-block: clamp(3.75rem, 8vw, 7rem);
	}
	.feature-section {
		padding-block: clamp(4.5rem, 9vw, 8rem);
	}

	.philosophy,
	.support-section {
		background: var(--landing-paper);
		border-top: 1px solid var(--border);
	}
	.philosophy {
		border-bottom: 1px solid var(--border);
		text-align: center;
		padding-block: clamp(5rem, 12vw, 12rem);
	}
	.philosophy h2 {
		font-size: clamp(3rem, 7.1vw, 7rem);
	}
	.philosophy p,
	.invitation p {
		margin: 1.25rem auto 0;
		max-width: 38ch;
		font-size: clamp(1.05rem, 1.65vw, 1.3rem);
		line-height: 1.45;
		text-wrap: balance;
	}
	.philosophy p {
		max-width: 60rem;
		color: var(--landing-ink);
	}

	.atmosphere-section {
		grid-template-columns: minmax(0, 1fr) minmax(0, 1.1fr);
		align-items: center;
		gap: 2rem clamp(1.75rem, 5.5vw, 6rem);
		padding-block: clamp(4.5rem, 9vw, 8.5rem);
	}
	.atmosphere-section h2 {
		margin: 0.65rem 0 1.3rem;
	}
	.section-name {
		color: var(--landing-ink);
		font-size: 0.9rem;
		font-weight: 600;
	}
	.text-link {
		display: inline-flex;
		align-items: center;
		gap: 0.35rem;
		margin-top: 1.15rem;
		padding-block: 0.3rem;
		text-decoration: underline;
		text-underline-offset: 0.25rem;
	}
	.text-link:hover {
		color: var(--landing-ink);
	}

	.support-section {
		padding-block: clamp(3.5rem, 7.5vw, 6.5rem) clamp(4rem, 8vw, 7rem);
	}
	.support-section .split {
		align-items: center;
	}
	.support-section .landing-heading {
		font-size: clamp(2.25rem, 4vw, 3.375rem);
		text-wrap: balance;
	}

	.invitation {
		position: relative;
		isolation: isolate;
		display: grid;
		align-items: center;
		min-height: clamp(26rem, 42vw, 38rem);
		padding-block: 4rem;
		background: #0b1933;
		color: #fff;
		overflow: hidden;
		--primary: #f7f9fa;
		--primary-foreground: #13181d;
	}
	.invitation::after,
	.invitation img {
		position: absolute;
		inset: 0;
		z-index: -1;
		width: 100%;
		height: 100%;
	}
	.invitation img {
		object-fit: cover;
		object-position: center 54%;
	}
	.invitation::after {
		content: '';
		background: linear-gradient(90deg, #0b1328f0, #0b13288a 56%, #0b13282e);
	}
	.invitation h2 {
		max-width: 13ch;
		text-wrap: pretty;
	}
	.invitation p {
		margin: 1rem 0 1.55rem;
	}

	@media (max-width: 720px) {
		.split {
			grid-template-columns: 1fr;
		}
		.split h2 {
			max-width: 15ch;
		}
		.invitation img {
			object-position: 60% center;
		}
		.invitation::after {
			background: #0b1328a8;
		}
	}
</style>

<script lang="ts">
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import Folder from '@lucide/svelte/icons/folder';
	import Search from '@lucide/svelte/icons/search';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu';
	import { Button } from '$lib/components/ui/button';
	import SupporterBadge from '$lib/components/supporter-badge.svelte';
	import { FEED_LEVELS, findFeedLevel, type FeedLevelSlug } from '$lib/feed-levels';
	import LandingImage, { type Reference } from './landing-image.svelte';

	const palettes: { name: string; hex: string; images: Reference[] }[] = [
		{
			name: 'Indigo',
			hex: '#253d77',
			images: ['indigo-building', 'indigo-tiles', 'indigo-curtain']
		},
		{ name: 'Vermilion', hex: '#c43624', images: ['red-grating', 'red-lantern', 'red-lights'] },
		{ name: 'Moss', hex: '#556846', images: ['moss-sprigs', 'moss-carpet', 'moss-glass'] }
	];
	let selectedColor = $state(0);
	let palette = $derived(palettes[selectedColor]);

	const feeds: Record<FeedLevelSlug, { description: string; images: Reference[] }> = {
		personal: {
			description: 'Closer to the things you already save.',
			images: ['feed-chair', 'feed-bedroom', 'feed-interior']
		},
		general: {
			description: 'Discover popular recent saves in a shared feed.',
			images: ['feed-stair', 'feed-blossoms', 'feed-coast']
		},
		'new-worlds': {
			description: 'Unexplored interests near the things you save.',
			images: ['feed-bowls', 'feed-canyon', 'feed-lamps']
		}
	};
	let feedLevel = $state<FeedLevelSlug>('general');
	let feed = $derived(feeds[feedLevel]);
</script>

{#snippet imageStrip(images: Reference[], className = '')}
	<div class="image-strip {className}">
		{#each images as id (id)}
			<LandingImage {id} />
		{/each}
	</div>
{/snippet}

{#snippet supporterNote()}
	<span class="supporter-note">
		<span aria-hidden="true"><SupporterBadge class="size-3.5" /></span>
		Supporter feature
	</span>
{/snippet}

<section aria-labelledby="landing-features">
	<h2 id="landing-features" class="landing-heading">A toolkit for your creativity.</h2>

	<div class="bento">
		<article class="semantic wide">
			<div class="tile-copy">
				<h3>Semantic search</h3>
				<p>Describe an idea, a mood or a scene you can picture.</p>
			</div>
			<div class="search-example">
				<div class="example-query">
					<Search class="size-4 shrink-0" /> <span>Quiet interiors with warm afternoon light</span>
				</div>
				{@render imageStrip(['afternoon-room', 'afternoon-table'])}
			</div>
		</article>

		<article class="color" style:--sample-color={palette.hex}>
			<div class="tile-copy">
				<h3>Color search</h3>
				<p>Follow a hue through different subjects.</p>
				{@render supporterNote()}
			</div>
			{@render imageStrip(palette.images, 'color-study')}
			<div class="palette">
				<div class="swatches" role="group" aria-label="Choose an example color">
					{#each palettes as { name, hex }, index (hex)}
						<button
							type="button"
							aria-label={name}
							aria-pressed={index === selectedColor}
							style:--swatch={hex}
							onclick={() => (selectedColor = index)}
						>
							{#if index === selectedColor}<Check class="size-4" />{/if}
						</button>
					{/each}
				</div>
				<span aria-live="polite">{palette.name}</span>
			</div>
		</article>

		<article class="related">
			<div class="tile-copy">
				<h3>Related images</h3>
				<p>See where a visual resemblance takes you.</p>
			</div>
			{@render imageStrip(
				['related-fern', 'related-pattern', 'related-fiddlehead'],
				'related-study'
			)}
		</article>

		<article class="personal">
			<div class="tile-copy">
				<h3>Personalized feeds</h3>
				<p aria-live="polite">{feed.description}</p>
			</div>
			<div class="feed-example">
				{@render imageStrip(feed.images)}
				<div class="feed-control">
					<DropdownMenu.Root>
						<DropdownMenu.Trigger>
							{#snippet child({ props })}
								<Button
									{...props}
									variant="outline"
									class="h-10 gap-2 bg-background dark:bg-background"
									aria-label="Choose an example feed"
								>
									{findFeedLevel(feedLevel)?.label}
									<ChevronDown class="size-3.5" />
								</Button>
							{/snippet}
						</DropdownMenu.Trigger>
						<DropdownMenu.Content align="end">
							<DropdownMenu.RadioGroup bind:value={feedLevel}>
								{#each FEED_LEVELS as { slug, label } (slug)}
									<DropdownMenu.RadioItem value={slug}>{label}</DropdownMenu.RadioItem>
								{/each}
							</DropdownMenu.RadioGroup>
						</DropdownMenu.Content>
					</DropdownMenu.Root>
				</div>
			</div>
		</article>

		<article class="saving">
			<div class="tile-copy">
				<h3>Save and collect</h3>
				<p>Keep images from across the web with the browser extension.</p>
			</div>
			<div class="save-example">
				<LandingImage id="rhinoceros-print" />
				<span class="example-query"
					><Folder class="size-4 shrink-0" /> Printed matter <Check class="size-4 shrink-0" /></span
				>
			</div>
		</article>

		<article class="library wide">
			<div class="tile-copy">
				<h3>Library search</h3>
				<p>Find a save by what you remember about it.</p>
				{@render supporterNote()}
			</div>
			<div class="search-example">
				<div class="example-query">
					<Search class="size-4 shrink-0" /> <span>Blue textures and folds</span><span class="scope"
						>My library</span
					>
				</div>
				{@render imageStrip(['library-weave', 'library-denim'])}
			</div>
		</article>
	</div>
</section>

<style>
	h2 {
		max-width: 15ch;
		margin-bottom: clamp(1.75rem, 4vw, 3rem);
	}

	.bento {
		display: grid;
		grid-template-columns: repeat(3, minmax(0, 1fr));
		grid-template-areas:
			'semantic semantic color'
			'related personal color'
			'saving library library';
		gap: clamp(0.65rem, 1.2vw, 1rem);
	}

	article {
		display: grid;
		grid-template-rows: auto 1fr;
		gap: 1rem;
		min-width: 0;
		padding: clamp(0.9rem, 1.8vw, 1.4rem);
		border: 1px solid var(--border);
		border-radius: var(--radius-xl);
		background: var(--landing-paper);
	}
	.semantic {
		grid-area: semantic;
	}
	.color {
		grid-area: color;
	}
	.related {
		grid-area: related;
	}
	.personal {
		grid-area: personal;
	}
	.saving {
		grid-area: saving;
	}
	.library {
		grid-area: library;
	}

	.tile-copy h3 {
		font-size: clamp(1.35rem, 2.1vw, 1.8rem);
		font-weight: 600;
		letter-spacing: -0.035em;
		line-height: 1.1;
	}
	.tile-copy p {
		margin-top: 0.35rem;
		color: var(--landing-ink);
		font-size: clamp(0.9rem, 1.15vw, 1rem);
		line-height: 1.35;
		text-wrap: pretty;
	}
	.supporter-note {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		margin-top: 0.6rem;
		color: var(--landing-ink);
		font-size: 0.78rem;
	}

	.wide {
		grid-template: 1fr / minmax(0, 0.8fr) minmax(0, 1.2fr);
		min-height: 15rem;
	}
	.wide .tile-copy {
		align-self: center;
	}
	.wide .tile-copy p {
		max-width: 24ch;
	}

	.search-example {
		align-self: center;
		display: grid;
		grid-template-rows: auto 1fr;
		min-width: 0;
		gap: 0.4rem;
	}
	.example-query {
		display: flex;
		align-items: center;
		gap: 0.45rem;
		min-width: 0;
		padding: 0.55rem 0.65rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--background);
		color: var(--foreground);
		font-size: 0.8rem;
		line-height: 1.2;
	}
	.scope {
		margin-left: auto;
		flex-shrink: 0;
		color: var(--landing-ink);
		font-size: 0.72rem;
	}

	.image-strip {
		display: grid;
		grid-auto-flow: column;
		grid-auto-columns: minmax(0, 1fr);
		gap: 0.25rem;
		min-height: 0;
		overflow: hidden;
		border-radius: var(--radius);
	}
	.image-strip :global(img) {
		height: 100%;
		min-height: 0;
		min-width: 0;
	}
	.search-example .image-strip {
		height: clamp(7rem, 13vw, 11.5rem);
	}
	.related-study {
		height: 7rem;
		grid-template-columns: 1.45fr 1fr 1fr;
	}

	.color {
		grid-template-rows: auto 1fr auto;
		background: color-mix(in srgb, var(--landing-paper) 84%, var(--sample-color));
	}
	.color-study {
		grid-auto-flow: row;
		grid-template-columns: 1.35fr 1fr;
		grid-template-rows: repeat(2, minmax(0, 1fr));
		min-height: 9rem;
	}
	.color-study :global(img:first-child) {
		grid-row: span 2;
	}
	.palette,
	.swatches {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 0.45rem;
	}
	.palette {
		gap: 0.6rem;
		font-size: 0.84rem;
	}
	.swatches button {
		display: grid;
		place-items: center;
		width: 2.75rem;
		height: 2.75rem;
		border: 2px solid transparent;
		border-radius: 50%;
		background: var(--swatch);
		background-clip: padding-box;
		color: white;
		cursor: pointer;
	}
	.swatches button[aria-pressed='true'] {
		border-color: var(--foreground);
	}
	.swatches button:focus-visible {
		outline: 2px solid var(--foreground);
		outline-offset: 2px;
	}

	.feed-example,
	.save-example {
		position: relative;
		min-width: 0;
	}
	.feed-example {
		align-self: end;
	}
	.feed-example .image-strip {
		height: 7rem;
	}
	.feed-control {
		position: absolute;
		right: 0.45rem;
		bottom: 0.45rem;
	}
	.save-example :global(img) {
		height: 8.5rem;
		border-radius: var(--radius);
		object-position: center 45%;
	}
	.save-example .example-query {
		position: absolute;
		right: 0.45rem;
		bottom: 0.45rem;
	}

	@media (max-width: 900px) {
		.bento {
			grid-template-columns: repeat(2, minmax(0, 1fr));
			grid-template-areas:
				'semantic semantic'
				'color related'
				'personal saving'
				'library library';
		}
		.related-study {
			height: auto;
		}
		.color-study {
			height: 9rem;
		}
	}
	@media (max-width: 560px) {
		.bento {
			grid-template-columns: minmax(0, 1fr);
			grid-template-areas: none;
		}
		.bento > article {
			grid-area: auto;
		}
		.wide {
			grid-template: auto 1fr / minmax(0, 1fr);
			min-height: 0;
		}
		.wide .tile-copy p {
			max-width: none;
		}
		.related-study {
			height: 7rem;
		}
	}
</style>

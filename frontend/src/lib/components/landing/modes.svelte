<script lang="ts">
	import Compass from '@lucide/svelte/icons/compass';
	import Folder from '@lucide/svelte/icons/folder';
	import FolderPlus from '@lucide/svelte/icons/folder-plus';
	import ListFilter from '@lucide/svelte/icons/list-filter';
	import PanelLeft from '@lucide/svelte/icons/panel-left';
	import Search from '@lucide/svelte/icons/search';
	import X from '@lucide/svelte/icons/x';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import * as Tabs from '$lib/components/ui/tabs';
	import LogoIcon from '$lib/assets/logo-icon.svelte';
	import Plus from '@lucide/svelte/icons/plus';
	import Equal from '@lucide/svelte/icons/equal';
	import { prefersReducedMotion } from 'svelte/motion';
	import LandingImage, { type Reference } from './landing-image.svelte';

	const explore: Reference[][] = [
		['concrete-facade', 'sea-steps', 'studio-textiles', 'cut-grapefruit'],
		['field-poppies', 'window-seat', 'type-keys', 'cloud-hills'],
		['golden-woods', 'paper-fold', 'glass-marbles', 'coastal-foam'],
		['red-door', 'blue-bottles', 'concrete-angles'],
		['olive-silk', 'leaf-veins', 'dark-tunnel']
	];
	const library: Reference[] = ['quiet-armchair', 'quiet-table', 'quiet-ceramic', 'quiet-window'];

	const modes = [
		{
			id: 'explore',
			label: 'Explore',
			title: 'Discover new worlds.',
			description: 'Discover new images, follow your interests and see what one picture leads to.'
		},
		{
			id: 'organize',
			label: 'Organize',
			title: 'Give it a place to live.',
			description:
				'Group saves into collections and sections, edit their details and find them whenever you need them.'
		}
	] as const;
	type Mode = (typeof modes)[number]['id'];

	let scrollProgress = $state(0);
	let chosenMode = $state<Mode>();
	const value = $derived(chosenMode ?? (scrollProgress < 0.5 ? 'explore' : 'organize'));
	const progress = $derived(
		chosenMode || prefersReducedMotion.current ? Number(value === 'organize') : scrollProgress
	);

	function trackScroll(section: HTMLElement) {
		const stage = section.firstElementChild as HTMLElement;
		let frame = 0;

		function update() {
			frame = 0;
			const rect = section.getBoundingClientRect();
			if (rect.bottom <= 0 || rect.top >= window.innerHeight) {
				chosenMode = undefined;
				scrollProgress = Number(rect.bottom <= 0);
				return;
			}
			const stickyTop = parseFloat(getComputedStyle(stage).top);
			const distance = rect.height - stage.offsetHeight;
			// Give both completed views time on screen before and after the crossfade.
			const next = (stickyTop - rect.top - distance * 0.24) / (distance * 0.52);
			scrollProgress = Math.max(0, Math.min(1, next));
		}

		function schedule() {
			if (!frame) frame = requestAnimationFrame(update);
		}

		const observer = new ResizeObserver(schedule);
		observer.observe(section);
		window.addEventListener('scroll', schedule, { passive: true });
		schedule();

		return () => {
			cancelAnimationFrame(frame);
			observer.disconnect();
			window.removeEventListener('scroll', schedule);
		};
	}
</script>

{#snippet exploreView()}
	<div class="explore-preview">
		<div class="explore-grid">
			{#each explore as column (column[0])}
				<div>
					{#each column as id (id)}
						<LandingImage {id} decorative />
					{/each}
				</div>
			{/each}
		</div>
		<span class="control feed"><Compass class="size-4" /> General</span>
	</div>
{/snippet}

{#snippet organizeView()}
	<div class="organize-preview">
		<div class="library">
			<div class="control create"><FolderPlus class="size-3.5" /> Create collection</div>
			<div class="control filter"><ListFilter class="size-3.5" /> Filter collections…</div>
			<div class="library-label">My library</div>
			<div class="library-item">Unsorted</div>
			{#each ['Quiet rooms', 'Printed matter', 'Along the coast'] as name (name)}
				<div class={['library-item', { current: name === 'Quiet rooms' }]}>
					<Folder class="size-3.5 shrink-0" />
					{name}
				</div>
			{/each}
			<div class="library-label">Favourites</div>
		</div>
		<div class="canvas">
			<div class="canvas-bar">
				<PanelLeft class="size-4 shrink-0 text-muted-foreground" />
				<span class="breadcrumb">My library <ChevronRight class="size-3" /></span>
				<strong>Quiet rooms</strong>
				<span class="control canvas-search"
					><Search class="size-3.5" /> <span>Search your library…</span></span
				>
			</div>
			<div class="organize-grid">
				{#each library as id (id)}
					<LandingImage {id} decorative class={id === 'quiet-armchair' ? 'selected-save' : ''} />
				{/each}
			</div>
		</div>
		<div class="details">
			<div class="details-header">
				<div class="detail-tabs"><strong>Details</strong><span>Related</span></div>
				<X class="size-3.5" />
			</div>
			<LandingImage id="quiet-armchair" decorative />
			<div class="detail-fields">
				<div class="detail-field">
					<strong>Palette</strong>
					<div class="detail-palette" aria-hidden="true">
						{#each ['#f1d5a6', '#c2945d', '#956433', '#624020', '#3c2a17'] as hex (hex)}
							<span style:background={hex}></span>
						{/each}
					</div>
				</div>
				<div class="detail-field">
					<strong>In your collections</strong>
					<span class="collection-chip"><Folder class="size-3" /> Quiet rooms</span>
				</div>
				<div class="detail-field">
					<strong>Notes</strong>
					<p>Warm neutrals and a small pop of yellow.</p>
				</div>
				<div class="detail-field">
					<strong>Source</strong>
					<p>pxhere.com</p>
				</div>
				<div class="detail-field">
					<strong>Info</strong>
					<dl>
						<div>
							<dt>Dimensions</dt>
							<dd>360 × 360</dd>
						</div>
						<div>
							<dt>Saved by</dt>
							<dd>@marin.example</dd>
						</div>
						<div>
							<dt>Saved on</dt>
							<dd>Oct 12, 2026</dd>
						</div>
					</dl>
				</div>
				<div class="detail-field">
					<strong>Alt text</strong>
					<p>A pale armchair beside dark branches and yellow flowers.</p>
				</div>
			</div>
		</div>
	</div>
{/snippet}

<section class="landing-modes" aria-label="Explore and Organize" {@attach trackScroll}>
	<div
		class="stage"
		style:--landing-mode-progress={progress}
		data-manual={chosenMode ? '' : undefined}
	>
		<div class="preview-copy">
			{#each modes as mode (mode.id)}
				<div class="copy {mode.id}" aria-hidden={value !== mode.id}>
					<h2 class="landing-heading">{mode.title}</h2>
					<p>{mode.description}</p>
				</div>
			{/each}
		</div>

		<Tabs.Root {value} onValueChange={(next) => (chosenMode = next as Mode)} class="preview gap-0">
			<div class="toolbar">
				<span class="preview-logo" aria-hidden="true"><LogoIcon /></span>
				<Tabs.List
					aria-label="Currents modes"
					class="switcher h-auto p-0.75 group-data-horizontal/tabs:h-auto"
				>
					<span aria-hidden="true" class="indicator"></span>
					{#each modes as mode (mode.id)}
						<Tabs.Trigger
							value={mode.id}
							class="h-8 min-w-0 px-2.5 text-xs data-active:bg-transparent! dark:data-active:border-transparent! dark:data-active:bg-transparent!"
							>{mode.label}</Tabs.Trigger
						>
					{/each}
				</Tabs.List>
				<div class="preview-actions" aria-hidden="true">
					<Search />
					<Plus />
					<Equal />
					<LandingImage id="profile-flower" decorative class="preview-avatar" />
				</div>
			</div>
			<div class="views">
				{#each modes as mode (mode.id)}
					<Tabs.Content value={mode.id}>
						{#snippet child({ props })}
							<div {...props} hidden={false} inert={value !== mode.id} class="view {mode.id}">
								{#if mode.id === 'explore'}
									<p class="sr-only">A masonry feed with search and a feed selector.</p>
									{@render exploreView()}
								{:else}
									<p class="sr-only">A workspace for collections, saves and image details.</p>
									{@render organizeView()}
								{/if}
							</div>
						{/snippet}
					</Tabs.Content>
				{/each}
			</div>
		</Tabs.Root>
	</div>
</section>

<style>
	@property --landing-mode-progress {
		syntax: '<number>';
		inherits: true;
		initial-value: 0;
	}

	.landing-modes::after {
		content: '';
		display: block;
		height: clamp(19rem, 60svh, 36rem);
	}

	.stage {
		--stage-top: calc(env(safe-area-inset-top) + 5.75rem);
		position: sticky;
		top: var(--stage-top);
		display: grid;
		grid-template-rows: auto minmax(0, 1fr);
		gap: clamp(0.85rem, 2vw, 1.6rem);
		height: min(calc(100svh - var(--stage-top) - 0.85rem), clamp(30rem, 54vw, 43rem));
	}

	.preview-copy {
		display: grid;
	}
	.copy {
		grid-area: 1 / 1;
		display: grid;
		grid-template-columns: minmax(0, 1.3fr) minmax(0, 0.7fr);
		align-items: end;
		gap: 0.6rem 2rem;
	}
	.copy h2 {
		text-wrap: pretty;
	}
	.copy p {
		max-width: 37ch;
		color: var(--landing-ink);
		font-size: clamp(1rem, 1.5vw, 1.2rem);
		line-height: 1.4;
		text-wrap: pretty;
	}
	.copy.explore {
		opacity: clamp(0, calc(1 - var(--landing-mode-progress) * 2), 1);
	}
	.copy.organize {
		opacity: clamp(0, calc(var(--landing-mode-progress) * 2 - 1), 1);
	}

	.stage :global(.preview) {
		--mock-toolbar-height: clamp(3.15rem, 6vw, 3.85rem);
		position: relative;
		min-height: 0;
		max-height: 80svh;
		overflow: hidden;
		border: 1px solid var(--border);
		border-radius: var(--radius-xl);
		background: var(--background);
	}
	.toolbar {
		position: absolute;
		inset: 0 0 auto;
		height: var(--mock-toolbar-height);
		pointer-events: none;
		display: flex;
		align-items: center;
		gap: 0.45rem;
		padding: clamp(0.35rem, 0.85vw, 0.65rem);
		z-index: 2;
	}
	.preview-logo,
	.preview-actions,
	.toolbar :global(.switcher) {
		flex-shrink: 0;
		border: 1px solid var(--border);
		border-radius: 999px;
		background: var(--card);
	}
	.preview-logo {
		display: grid;
		place-items: center;
		width: 2.15rem;
		height: 2.15rem;
		padding: 0.26rem;
	}
	.toolbar :global(.switcher) {
		position: relative;
		pointer-events: auto;
		width: clamp(8rem, 14vw, 10.625rem);
	}
	.indicator {
		position: absolute;
		inset: 3px auto 3px 3px;
		width: calc((100% - 6px) / 2);
		border-radius: 999px;
		border: 1px solid var(--border);
		background: var(--secondary);
		transform: translateX(calc(100% * var(--landing-mode-progress)));
		pointer-events: none;
	}
	.preview-actions {
		display: flex;
		align-items: center;
		gap: clamp(0.35rem, 1vw, 0.9rem);
		margin-left: auto;
		padding: 0.27rem 0.37rem 0.27rem 0.52rem;
		opacity: calc(1 - var(--landing-mode-progress));
	}
	.preview-actions :global(svg) {
		width: clamp(0.75rem, 1.5vw, 1rem);
		height: auto;
	}
	.preview-actions :global(.preview-avatar) {
		width: clamp(1.15rem, 2.2vw, 1.65rem);
		aspect-ratio: 1;
		border-radius: 50%;
		object-position: center top;
	}

	.views {
		position: relative;
		flex: 1;
		min-height: 0;
	}
	.view {
		position: absolute;
		inset: 0;
		outline: none;
	}
	.view:focus-visible {
		outline: 2px solid var(--ring);
		outline-offset: -2px;
		z-index: 1;
	}
	.view.organize {
		opacity: var(--landing-mode-progress);
	}
	.explore-preview,
	.organize-preview {
		position: relative;
		height: 100%;
		overflow: hidden;
	}

	@media (prefers-reduced-motion: no-preference) {
		.stage[data-manual] {
			transition: --landing-mode-progress 240ms ease-out;
		}
	}

	.control {
		display: inline-flex;
		align-items: center;
		gap: 0.45rem;
		padding: 0.45rem 0.65rem;
		border: 1px solid var(--border);
		border-radius: 2rem;
		background: var(--background);
		color: var(--foreground);
		line-height: 1.1;
	}

	.explore-preview {
		padding-top: var(--mock-toolbar-height);
	}
	.explore-grid {
		display: grid;
		grid-template-columns: repeat(5, minmax(0, 1fr));
		gap: 0.5rem;
		padding: 0.55rem;
	}
	.explore-grid > div {
		display: grid;
		align-content: start;
		gap: 0.5rem;
	}
	.explore-grid :global(img) {
		aspect-ratio: 0.92;
		border-radius: var(--radius);
	}
	.explore-grid > :nth-child(2n) :global(img:first-child),
	.explore-grid > :nth-child(2n + 1) :global(img:last-child) {
		aspect-ratio: 0.76;
	}
	.explore-grid > :nth-child(3) :global(img:first-child) {
		aspect-ratio: 1.22;
	}
	.feed {
		position: absolute;
		right: 1rem;
		bottom: 0.9rem;
		font-size: 0.875rem;
	}

	.organize-preview {
		display: grid;
		grid-template-columns: minmax(13rem, 20%) minmax(0, 1fr) minmax(10.5rem, 24%);
		font-size: clamp(0.67rem, 1.3vw, 0.9rem);
		background: var(--landing-paper);
	}
	.library,
	.details {
		padding: clamp(0.55rem, 1.3vw, 1.1rem);
	}
	.library {
		padding-top: calc(var(--mock-toolbar-height) + 0.45rem);
	}
	.library-item,
	.canvas-bar,
	.details-header,
	.breadcrumb {
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}
	.create,
	.filter {
		display: flex;
		margin-bottom: 0.45rem;
	}
	.create {
		background: var(--primary);
		color: var(--primary-foreground);
	}
	.filter,
	.library-label,
	.breadcrumb {
		color: var(--landing-ink);
	}
	.library-label {
		margin: 1.2rem 0 0.5rem;
		font-size: 0.9em;
	}
	.library-item {
		padding: 0.42rem 0.4rem;
		border-radius: var(--radius-sm);
	}
	.library-item.current {
		background: var(--secondary);
		font-weight: 600;
	}

	.canvas {
		min-width: 0;
		margin-block: 0.45rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--background);
	}
	.canvas-bar {
		min-height: 3rem;
		padding: 0.4rem 0.65rem;
		border-bottom: 1px solid var(--border);
	}
	.canvas-search {
		margin-left: auto;
		white-space: nowrap;
	}
	.organize-grid {
		display: grid;
		grid-template-columns: repeat(2, minmax(0, 1fr));
		gap: 0.35rem;
		padding: 0.55rem;
	}
	.organize-grid :global(img),
	.details > :global(img) {
		border-radius: var(--radius-sm);
		aspect-ratio: 1.08;
	}
	.organize-grid :global(.selected-save) {
		outline: 2px solid var(--foreground);
		outline-offset: -2px;
	}
	.details {
		min-width: 0;
		overflow: hidden;
		border-left: 1px solid var(--border);
		background: var(--sidebar);
		font-size: clamp(0.7rem, 1.04vw, 0.8rem);
		line-height: 1.3;
	}
	.details-header {
		justify-content: space-between;
		margin-bottom: 0.55rem;
	}
	.detail-tabs {
		display: flex;
		gap: 0.55rem;
		align-items: center;
		padding: 0.17rem 0.5rem 0.17rem 0.17rem;
		border-radius: 999px;
		background: var(--muted);
		color: var(--landing-ink);
	}
	.detail-tabs strong {
		padding: 0.22rem 0.48rem;
		border-radius: 999px;
		background: var(--background);
		color: var(--foreground);
	}
	.details > :global(img) {
		height: clamp(6rem, 11vw, 8.5rem);
	}
	.detail-fields {
		display: grid;
		gap: 0.4rem;
		margin-top: 0.55rem;
	}
	.detail-field {
		min-width: 0;
		display: grid;
		align-content: start;
		gap: 0.24rem;
		padding: 0.4rem 0.52rem;
		border: 1px solid var(--border);
		border-radius: var(--radius);
		background: var(--card);
	}
	.detail-field strong {
		font-weight: 600;
	}
	.detail-field p,
	.detail-field dt {
		color: var(--landing-ink);
	}
	.detail-field dl > div {
		display: flex;
		flex-wrap: wrap;
		justify-content: space-between;
		gap: 0 0.25rem;
	}
	.detail-palette {
		display: flex;
		height: 0.85rem;
		overflow: hidden;
		border-radius: 0.2rem;
	}
	.detail-palette span {
		flex: 1;
	}
	.collection-chip {
		display: flex;
		width: fit-content;
		align-items: center;
		gap: 0.3rem;
		padding: 0.15rem 0.4rem;
		border-radius: 999px;
		background: var(--muted);
	}

	@media (max-width: 820px) {
		.breadcrumb,
		.canvas-search span {
			display: none;
		}
	}
	@media (max-width: 700px) {
		.stage {
			height: min(calc(100svh - var(--stage-top) - 1.25rem), 39rem);
		}
		.copy {
			grid-template-columns: 1fr;
			gap: 0.45rem;
		}
		.preview-copy .copy h2 {
			font-size: clamp(1.85rem, 7.5vw, 2.55rem);
		}
		.copy p {
			font-size: 0.95rem;
		}
		.toolbar {
			gap: 0.3rem;
		}
		.explore-grid {
			grid-template-columns: repeat(3, minmax(0, 1fr));
		}
		.explore-grid > :nth-child(n + 4),
		.library,
		.canvas,
		.details-header {
			display: none;
		}
		.organize-preview {
			grid-template-columns: minmax(0, 1fr);
		}
		.details {
			border-left: 0;
			padding-top: var(--mock-toolbar-height);
		}
		.details > :global(img) {
			height: clamp(7rem, 51vw, 14rem);
		}
		.detail-fields {
			grid-template-columns: repeat(2, minmax(0, 1fr));
		}
	}
</style>

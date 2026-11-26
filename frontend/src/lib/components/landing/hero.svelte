<script lang="ts">
	import type { Snippet } from 'svelte';
	import LandingImage, { type Reference } from './landing-image.svelte';

	let { action }: { action: Snippet } = $props();

	const columns: {
		offset: number;
		drift: number;
		minWidth?: number;
		images: [Reference, number][];
	}[] = [
		{
			offset: -57,
			drift: 92,
			images: [
				['blue-water', 0.92],
				['red-drapery', 0.69],
				['pottery-wheel', 1.2],
				['lit-moss', 0.9],
				['shore-pebbles', 1.1]
			]
		},
		{
			offset: -27,
			drift: -58,
			minWidth: 850,
			images: [
				['pale-stairs', 0.78],
				['sunlit-chair', 1.16],
				['fern', 0.85],
				['spiral-stair', 0.82],
				['golden-facade', 1.1]
			]
		},
		{
			offset: -108,
			drift: 64,
			images: [
				['poppies', 0.65],
				['sand-ripples', 1.48],
				['letterpress', 0.68],
				['glass-sphere', 1.1],
				['pink-anemone', 0.82]
			]
		},
		{
			offset: -20,
			drift: -82,
			images: [
				['indigo-silk', 1.44],
				['sunlit-sofa', 0.76],
				['violet-ridge', 0.8],
				['amber-study', 0.9],
				['frost-detail', 1.14]
			]
		},
		{
			offset: -62,
			drift: 52,
			minWidth: 1120,
			images: [
				['color-study', 0.74],
				['celadon', 1.1],
				['woven-textile', 0.88],
				['glazed-teapot', 1.16],
				['blue-horizon', 0.8]
			]
		},
		{
			offset: -21,
			drift: -62,
			images: [
				['misty-forest', 0.91],
				['blue-pier', 0.74],
				['persimmons', 1.06],
				['sandstone-arch', 0.92],
				['cloud-sea', 1.18]
			]
		}
	];

	function pointerParallax(hero: HTMLElement) {
		const motion = matchMedia('(any-pointer: fine) and (prefers-reduced-motion: no-preference)');
		let frame = 0;

		function reset() {
			cancelAnimationFrame(frame);
			hero.style.removeProperty('--pointer-x');
			hero.style.removeProperty('--pointer-y');
		}

		function move(event: PointerEvent) {
			if (event.pointerType !== 'mouse' || !motion.matches) return;
			cancelAnimationFrame(frame);
			frame = requestAnimationFrame(() => {
				const { left, top, width, height } = hero.getBoundingClientRect();
				hero.style.setProperty('--pointer-x', `${((event.clientX - left) / width - 0.5) * 14}px`);
				hero.style.setProperty('--pointer-y', `${((event.clientY - top) / height - 0.5) * 8}px`);
			});
		}

		hero.addEventListener('pointermove', move, { passive: true });
		hero.addEventListener('pointerleave', reset);
		motion.addEventListener('change', reset);
		return () => {
			reset();
			hero.removeEventListener('pointermove', move);
			hero.removeEventListener('pointerleave', reset);
			motion.removeEventListener('change', reset);
		};
	}
</script>

<section class="hero" aria-labelledby="landing-title" {@attach pointerParallax}>
	<div class="wall" aria-hidden="true">
		{#each columns as { offset, drift, minWidth, images } (offset + ':' + drift)}
			{@const stackHeight = images.reduce((height, [, ratio]) => height + 1 / ratio, 0)}
			<div class="column" data-min-width={minWidth}>
				<div class="tiles" style:--offset="{offset}cqw" style:--drift="{drift}px">
					{#each images as [id, ratio], index (id)}
						<LandingImage
							{id}
							decorative
							eager={index < 2}
							minViewportWidth={minWidth}
							priority={id === 'sand-ripples'}
							style="aspect-ratio: {ratio}; --height-share: {1 / (stackHeight * ratio)}"
						/>
					{/each}
				</div>
			</div>
		{/each}
	</div>
	<div class="hero-copy">
		<h1 id="landing-title"><span>Get carried by</span> <span>the currents.</span></h1>
		<p>A calm space to discover, collect and organize visual inspiration.</p>
		{@render action()}
	</div>
</section>

<style>
	.hero {
		--hero-height: clamp(42rem, 57vw, 56rem);
		position: relative;
		isolation: isolate;
		display: grid;
		place-items: center;
		min-height: var(--hero-height);
		overflow: hidden;
		background: linear-gradient(to bottom, var(--landing-paper) 76%, var(--background));
		padding: 7rem 1rem 5rem;
		text-align: center;
	}

	.wall {
		--tile-gap: clamp(0.45rem, 1vw, 1rem);
		position: absolute;
		z-index: -1;
		inset: 0 -2.25%;
		display: grid;
		grid-template-columns: repeat(6, minmax(0, 1fr));
		gap: var(--tile-gap);
		align-items: start;
		/* Keep some texture behind the copy and fade the full wall into the page. */
		mask-image:
			linear-gradient(to bottom, #000a, #000e 15%, black 42%, #000d 74%, transparent),
			radial-gradient(ellipse 59% 58% at 50% 51%, #0002 0%, #0003 31%, #0007 59%, #000d 85%, black);
		mask-composite: intersect;
	}

	.column {
		container-type: inline-size;
		min-width: 0;
	}
	.tiles {
		display: grid;
		gap: var(--tile-gap);
		--column-offset: var(--offset);
		translate: 0 var(--column-offset);
	}

	.column :global(img) {
		border-radius: calc(var(--radius) * 0.85);
	}

	.hero-copy h1 {
		margin-inline: auto;
		font-size: clamp(3.65rem, 6.5vw, 7rem);
		font-weight: 590;
		line-height: 0.93;
		letter-spacing: -0.045em;
		word-spacing: 0.025em;
	}

	h1 span {
		display: block;
	}

	.hero-copy p {
		max-width: 29ch;
		margin: clamp(1.4rem, 3vw, 2.1rem) auto 1.65rem;
		font-size: clamp(1.05rem, 1.65vw, 1.42rem);
		line-height: 1.35;
		text-wrap: balance;
	}

	@media (prefers-reduced-motion: no-preference) {
		@supports (animation-timeline: scroll()) {
			.tiles {
				animation: drift linear both;
				animation-timeline: scroll(root block);
				animation-range: 0px 740px;
			}
		}
	}

	@media (any-pointer: fine) and (prefers-reduced-motion: no-preference) {
		.tiles {
			--pointer-depth: 0.55;
			translate: calc(var(--pointer-x, 0px) * var(--pointer-depth))
				calc(var(--column-offset) + var(--pointer-y, 0px) * var(--pointer-depth));
			transition: translate 400ms cubic-bezier(0.2, 0.65, 0.3, 1);
		}
		.column:nth-child(odd) .tiles {
			--pointer-depth: 1;
		}
	}

	@keyframes drift {
		from {
			transform: translateY(0);
		}
		to {
			transform: translateY(var(--drift));
		}
	}

	@media (max-width: 1120px) {
		.wall {
			inset-inline: -4%;
			grid-template-columns: repeat(5, minmax(0, 1fr));
		}
		.column[data-min-width='1120'] {
			display: none;
		}
	}

	@media (max-width: 850px) {
		.hero {
			--hero-height: 42rem;
		}
		.wall {
			inset-inline: -7%;
			grid-template-columns: repeat(4, minmax(0, 1fr));
		}
		.column[data-min-width='850'] {
			display: none;
		}
		.tiles {
			--column-offset: calc(var(--offset) / 2);
		}
	}

	@media (max-width: 580px) {
		.hero {
			--hero-height: clamp(36rem, 100svh, 42rem);
		}
		.wall {
			inset: -0.8rem -7% 0;
			mask-image: linear-gradient(
				to bottom,
				#0009,
				#000d 14%,
				#0007 31%,
				#0002 47%,
				#0003 65%,
				#000a 82%,
				#000b 89%,
				transparent
			);
		}
		.tiles {
			--column-offset: 0px;
		}
		.column :global(img) {
			/* Keep the proportions while leaving room for the columns to drift. */
			min-height: calc((var(--hero-height) + 6rem) * var(--height-share));
		}
		.hero-copy h1 {
			font-size: clamp(2.35rem, 10.6vw, 3.8rem);
			letter-spacing: -0.041em;
		}
		.hero-copy p {
			margin-bottom: 1.25rem;
		}
	}
</style>

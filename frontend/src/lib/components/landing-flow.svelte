<script lang="ts">
	let windowWidth = $state(typeof window !== 'undefined' ? window.innerWidth : 1024);

	$effect(() => {
		function onResize() {
			windowWidth = window.innerWidth;
		}
		window.addEventListener('resize', onResize);
		return () => window.removeEventListener('resize', onResize);
	});

	function getColumnCount(width: number): number {
		if (width >= 1536) return 7;
		if (width >= 1280) return 6;
		if (width >= 1024) return 5;
		if (width >= 768) return 4;
		if (width >= 480) return 3;
		return 2;
	}

	let columnCount = $derived(getColumnCount(windowWidth));

	// 7 columns, each with a distinct scroll speed and staggered delay.
	// Seeds are chosen to show varied, high-quality picsum subjects.
	const allColumns = [
		{
			duration: '44s',
			delay: '-18s',
			images: [
				{ seed: 115, h: 380 },
				{ seed: 126, h: 320 },
				{ seed: 133, h: 440 },
				{ seed: 141, h: 360 },
				{ seed: 155, h: 400 },
				{ seed: 168, h: 340 },
				{ seed: 177, h: 380 },
				{ seed: 189, h: 320 }
			]
		},
		{
			duration: '38s',
			delay: '-8s',
			images: [
				{ seed: 112, h: 340 },
				{ seed: 124, h: 420 },
				{ seed: 136, h: 360 },
				{ seed: 147, h: 400 },
				{ seed: 158, h: 320 },
				{ seed: 165, h: 380 },
				{ seed: 174, h: 340 },
				{ seed: 183, h: 420 }
			]
		},
		{
			duration: '52s',
			delay: '-30s',
			images: [
				{ seed: 110, h: 400 },
				{ seed: 122, h: 340 },
				{ seed: 131, h: 380 },
				{ seed: 144, h: 320 },
				{ seed: 153, h: 440 },
				{ seed: 162, h: 360 },
				{ seed: 173, h: 400 },
				{ seed: 185, h: 340 }
			]
		},
		{
			duration: '46s',
			delay: '-14s',
			images: [
				{ seed: 201, h: 360 },
				{ seed: 214, h: 420 },
				{ seed: 223, h: 340 },
				{ seed: 235, h: 400 },
				{ seed: 247, h: 360 },
				{ seed: 256, h: 440 },
				{ seed: 268, h: 320 },
				{ seed: 279, h: 380 }
			]
		},
		{
			duration: '35s',
			delay: '-5s',
			images: [
				{ seed: 302, h: 420 },
				{ seed: 315, h: 360 },
				{ seed: 328, h: 400 },
				{ seed: 341, h: 340 },
				{ seed: 354, h: 460 },
				{ seed: 367, h: 380 },
				{ seed: 378, h: 320 },
				{ seed: 391, h: 400 }
			]
		},
		{
			duration: '57s',
			delay: '-25s',
			images: [
				{ seed: 403, h: 340 },
				{ seed: 418, h: 400 },
				{ seed: 427, h: 360 },
				{ seed: 439, h: 440 },
				{ seed: 452, h: 320 },
				{ seed: 463, h: 380 },
				{ seed: 476, h: 400 },
				{ seed: 489, h: 340 }
			]
		},
		{
			duration: '41s',
			delay: '-20s',
			images: [
				{ seed: 501, h: 400 },
				{ seed: 514, h: 340 },
				{ seed: 527, h: 380 },
				{ seed: 538, h: 420 },
				{ seed: 549, h: 360 },
				{ seed: 562, h: 440 },
				{ seed: 575, h: 320 },
				{ seed: 588, h: 380 }
			]
		}
	] as const;

	let columns = $derived(allColumns.slice(0, columnCount));
</script>

<div class="flow-fade h-full w-full overflow-hidden">
	<div class="flow-stage">
		<div class="flex h-full gap-3 px-1.5">
			{#each columns as col (col.duration)}
				<div class="min-w-0 flex-1">
					<div
						class="track flex flex-col gap-3"
						style="animation-duration: {col.duration}; animation-delay: {col.delay};"
					>
						{#each [0, 1] as copy (copy)}
							{#each col.images as img (img.seed + '-' + copy)}
								<img
									src="https://picsum.photos/seed/{img.seed}/260/{img.h}"
									width="260"
									height={img.h}
									alt=""
									class="card w-full rounded-2xl"
									loading="eager"
									draggable="false"
								/>
							{/each}
						{/each}
					</div>
				</div>
			{/each}
		</div>
	</div>
</div>

<style>
	.flow-fade {
		perspective: 1800px;
		perspective-origin: 80% 40%;
		/* mask-image: linear-gradient(
			to bottom,
			black 0%,
			black 100%,
			transparent 100%
		);
		-webkit-mask-image: linear-gradient(
			to bottom,
			black 0%,
			black 100%,
			transparent 100%
		); */
	}

	.flow-stage {
		width: 100%;
		height: 100%;
		transform: rotateX(14deg) rotateY(-16deg) rotateZ(16deg) scale(1.55);
		transform-origin: center center;
		transform-style: preserve-3d;
	}

	@keyframes flow {
		from {
			transform: translateY(0);
		}
		to {
			transform: translateY(-50%);
		}
	}

	.track {
		animation: flow linear infinite;
		will-change: transform;
		transform-style: preserve-3d;
	}

	.card {
		box-shadow:
			-14px 22px 32px -14px rgba(20, 20, 30, 0.22),
			-4px 8px 14px -6px rgba(20, 20, 30, 0.14);
		backface-visibility: hidden;
	}
</style>

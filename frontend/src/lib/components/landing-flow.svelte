<script lang="ts">
	type CardSpec = {
		id: string;
		seed: number;
		width: number;
		height: number;
		x: number;
		y: number;
		rotation: number;
		scale: number;
		opacity: number;
		zIndex: number;
	};

	const cards: CardSpec[] = [
		{
			id: 'c1',
			seed: 118,
			width: 90,
			height: 116,
			x: 58,
			y: 16,
			rotation: -6,
			scale: 0.95,
			opacity: 0.84,
			zIndex: 12
		},
		{
			id: 'c2',
			seed: 133,
			width: 98,
			height: 126,
			x: 69,
			y: 11,
			rotation: 5,
			scale: 0.98,
			opacity: 0.88,
			zIndex: 14
		},
		{
			id: 'c3',
			seed: 147,
			width: 102,
			height: 132,
			x: 80,
			y: 18,
			rotation: -5,
			scale: 1,
			opacity: 0.91,
			zIndex: 16
		},
		{
			id: 'c4',
			seed: 169,
			width: 94,
			height: 120,
			x: 91,
			y: 13,
			rotation: 7,
			scale: 0.96,
			opacity: 0.86,
			zIndex: 18
		},
		{
			id: 'c5',
			seed: 184,
			width: 106,
			height: 138,
			x: 57,
			y: 36,
			rotation: 6,
			scale: 1.02,
			opacity: 0.9,
			zIndex: 20
		},
		{
			id: 'c6',
			seed: 208,
			width: 92,
			height: 118,
			x: 70,
			y: 33,
			rotation: -7,
			scale: 0.95,
			opacity: 0.84,
			zIndex: 22
		},
		{
			id: 'c7',
			seed: 224,
			width: 108,
			height: 140,
			x: 79,
			y: 40,
			rotation: 6,
			scale: 1.01,
			opacity: 0.92,
			zIndex: 24
		},
		{
			id: 'c8',
			seed: 241,
			width: 96,
			height: 124,
			x: 92,
			y: 35,
			rotation: -4,
			scale: 0.97,
			opacity: 0.87,
			zIndex: 26
		},
		{
			id: 'c9',
			seed: 259,
			width: 100,
			height: 128,
			x: 56,
			y: 58,
			rotation: -6,
			scale: 0.98,
			opacity: 0.88,
			zIndex: 28
		},
		{
			id: 'c10',
			seed: 274,
			width: 92,
			height: 120,
			x: 68,
			y: 56,
			rotation: 5,
			scale: 0.94,
			opacity: 0.83,
			zIndex: 30
		},
		{
			id: 'c11',
			seed: 291,
			width: 104,
			height: 134,
			x: 81,
			y: 61,
			rotation: -7,
			scale: 1.01,
			opacity: 0.9,
			zIndex: 32
		},
		{
			id: 'c12',
			seed: 304,
			width: 94,
			height: 122,
			x: 90,
			y: 57,
			rotation: 6,
			scale: 0.96,
			opacity: 0.85,
			zIndex: 34
		},
		{
			id: 'c13',
			seed: 318,
			width: 98,
			height: 126,
			x: 58,
			y: 80,
			rotation: 4,
			scale: 0.96,
			opacity: 0.84,
			zIndex: 36
		},
		{
			id: 'c14',
			seed: 333,
			width: 108,
			height: 138,
			x: 69,
			y: 77,
			rotation: -6,
			scale: 1.02,
			opacity: 0.91,
			zIndex: 38
		},
		{
			id: 'c15',
			seed: 347,
			width: 94,
			height: 120,
			x: 80,
			y: 84,
			rotation: 5,
			scale: 0.95,
			opacity: 0.86,
			zIndex: 40
		},
		{
			id: 'c16',
			seed: 361,
			width: 100,
			height: 128,
			x: 91,
			y: 79,
			rotation: -5,
			scale: 0.99,
			opacity: 0.88,
			zIndex: 42
		}
	];
</script>

<div class="flow-root" aria-hidden="true">
	{#each cards as card (card.id)}
		<img
			src="https://picsum.photos/seed/{card.seed}/{card.width * 4}/{card.height * 4}"
			width={card.width}
			height={card.height}
			alt=""
			class="flow-card"
			loading="eager"
			draggable="false"
			style="--card-x: {card.x}%; --card-y: {card.y}%; --card-width: {card.width}px; --card-height: {card.height}px; --card-scale: {card.scale}; --card-rotation: {card.rotation}deg; --card-opacity: {card.opacity}; --card-z: {card.zIndex};"
		/>
	{/each}
</div>

<style>
	.flow-root {
		position: relative;
		height: 100%;
		width: 100%;
		overflow: hidden;
		min-height: 0;
	}

	.flow-card {
		position: absolute;
		left: var(--card-x);
		top: var(--card-y);
		z-index: var(--card-z);
		height: var(--card-height);
		width: var(--card-width);
		border-radius: calc(var(--radius) * 2.5);
		border: 1px solid color-mix(in oklch, var(--border) 78%, transparent);
		object-fit: cover;
		opacity: var(--card-opacity);
		transform: translate(-50%, -50%) scale(var(--card-scale)) rotate(var(--card-rotation));
		transform-origin: center center;
		box-shadow:
			0 28px 60px -34px color-mix(in oklch, var(--foreground) 22%, transparent),
			0 12px 18px -16px color-mix(in oklch, var(--foreground) 18%, transparent);
		filter: saturate(0.92) contrast(1.02) brightness(0.97);
		user-select: none;
	}

	@media (max-width: 767px) {
		.flow-root {
			min-height: 16rem;
		}

		.flow-card {
			box-shadow:
				0 22px 44px -30px color-mix(in oklch, var(--foreground) 18%, transparent),
				0 10px 16px -14px color-mix(in oklch, var(--foreground) 16%, transparent);
		}
	}
</style>

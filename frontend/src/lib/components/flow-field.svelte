<script lang="ts">
	import { onMount } from 'svelte';

	interface Props {
		noiseIntensity?: number;
		noiseSpeed?: number;
		globalForce?: number;
		lineCount?: number;
		lineWeight?: number;
		waveSpeed?: number;
	}

	let {
		noiseIntensity = 1.5,
		noiseSpeed = 40,
		globalForce = 10,
		lineCount = 9,
		lineWeight = 1.5,
		waveSpeed = 10
	}: Props = $props();

	let canvas: HTMLCanvasElement;
	let container: HTMLDivElement;

	let time = 0;
	let noiseTime = 0;

	const hash = (n: number): number => {
		const x = Math.sin(n) * 43758.5453123;
		return x - Math.floor(x);
	};

	const lerp = (a: number, b: number, t: number): number =>
		a + (b - a) * (0.5 - Math.cos(t * Math.PI) * 0.5);

	const noise1D = (x: number): number => {
		const i = Math.floor(x);
		const f = x - i;
		return lerp(hash(i), hash(i + 1), f);
	};

	onMount(() => {
		const ctx = canvas.getContext('2d', { alpha: true });
		if (!ctx) return;

		let frame: number;

		const resize = (): void => {
			const dpr = window.devicePixelRatio || 1;
			canvas.width = container.offsetWidth * dpr;
			canvas.height = container.offsetHeight * dpr;
			ctx.scale(dpr, dpr);
		};

		const observer = new ResizeObserver(resize);
		observer.observe(container);
		resize();

		const render = (): void => {
			const w = container.offsetWidth;
			const h = container.offsetHeight;
			const centerX = w / 2;
			const centerY = h / 2;

			const radius = Math.min(w, h) * 0.42;

			ctx.clearRect(0, 0, w, h);

			const lineColor = getComputedStyle(canvas).getPropertyValue('--primary').trim() || '#00ff88';

			ctx.save();

			ctx.beginPath();
			ctx.arc(centerX, centerY, radius, 0, Math.PI * 2);
			ctx.clip();

			ctx.strokeStyle = lineColor;
			ctx.lineWidth = lineWeight;
			ctx.lineCap = 'round';

			const spacing = (radius * 2.2) / (lineCount - 1);

			for (let i = 0; i < lineCount; i++) {
				const xBase = centerX - radius * 1.1 + i * spacing;
				const lineSeed = i * 44.123;

				ctx.beginPath();
				for (let y = centerY - radius; y <= centerY + radius; y += 4) {
					const forceScale = (w / 100) * globalForce;
					const globalWave = Math.sin(y * 0.05 + time) * forceScale;

					const n1 = noise1D(y * 0.1 + lineSeed + noiseTime);
					const individualNoise = (n1 - 0.5) * (noiseIntensity * (w / 100));

					const xFinal = xBase + globalWave + individualNoise;

					if (y === centerY - radius) {
						ctx.moveTo(xFinal, y);
					} else {
						ctx.lineTo(xFinal, y);
					}
				}
				ctx.stroke();
			}

			ctx.restore();

			time += waveSpeed / 1000;
			noiseTime += noiseSpeed / 1000;
			frame = requestAnimationFrame(render);
		};

		render();

		return () => {
			observer.disconnect();
			cancelAnimationFrame(frame);
		};
	});
</script>

<div bind:this={container} class="flex h-full w-full items-center justify-center overflow-hidden">
	<canvas bind:this={canvas} class="block max-h-full max-w-full"></canvas>
</div>

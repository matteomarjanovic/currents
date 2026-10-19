// Inspired by https://transitions.dev/detail.html?t=delete-with-smoky-dissolve.
// Filter the live panel so images, text, and scrolled forms dissolve together.
export function smokyClose(panel: HTMLElement, duration: number) {
	const ns = 'http://www.w3.org/2000/svg';
	const svg = document.createElementNS(ns, 'svg');
	svg.setAttribute('aria-hidden', 'true');
	Object.assign(svg.style, {
		position: 'absolute',
		width: '0',
		height: '0',
		pointerEvents: 'none'
	});
	const filter = document.createElementNS(ns, 'filter');
	filter.id = 'currents-smoky-close';
	filter.setAttribute('x', '-30%');
	filter.setAttribute('y', '-30%');
	filter.setAttribute('width', '160%');
	filter.setAttribute('height', '160%');
	filter.setAttribute('color-interpolation-filters', 'sRGB');
	const noise = document.createElementNS(ns, 'feTurbulence');
	noise.setAttribute('type', 'fractalNoise');
	noise.setAttribute('baseFrequency', '0.012 0.018');
	noise.setAttribute('numOctaves', '2');
	noise.setAttribute('seed', '4');
	noise.setAttribute('result', 'smoke');
	const drift = document.createElementNS(ns, 'feOffset');
	drift.setAttribute('in', 'smoke');
	drift.setAttribute('result', 'drift');
	const warp = document.createElementNS(ns, 'feDisplacementMap');
	warp.setAttribute('in', 'SourceGraphic');
	warp.setAttribute('in2', 'drift');
	warp.setAttribute('xChannelSelector', 'R');
	warp.setAttribute('yChannelSelector', 'G');
	warp.setAttribute('scale', '0');
	const blur = document.createElementNS(ns, 'feGaussianBlur');
	blur.setAttribute('stdDeviation', '0');
	filter.append(noise, drift, warp, blur);
	svg.append(filter);
	panel.parentElement!.append(svg);
	panel.style.filter = `url(#${filter.id})`;

	const animation = panel.animate(
		[
			{ opacity: 1, transform: 'translateY(0) rotate(0deg)' },
			{ opacity: 0, transform: 'translateY(24px) rotate(1.5deg)' }
		],
		{ duration, easing: 'cubic-bezier(0.4, 0, 1, 1)', fill: 'forwards' }
	);
	let frame: number;
	const start = performance.now();
	function draw(now: number) {
		const p = Math.min((now - start) / duration, 1);
		warp.setAttribute('scale', String(15 * (1 - (1 - p) ** 3)));
		drift.setAttribute('dy', String(-24 * p));
		blur.setAttribute('stdDeviation', String(12 * p ** 2));
		if (p < 1) frame = requestAnimationFrame(draw);
	}
	frame = requestAnimationFrame(draw);
	return () => {
		cancelAnimationFrame(frame);
		animation.cancel();
		panel.style.removeProperty('filter');
		svg.remove();
	};
}

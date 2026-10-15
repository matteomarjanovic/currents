import { getContext, onDestroy, setContext, tick } from 'svelte';

type Navigate = (change: () => void, direction?: 1 | -1) => Promise<void>;
const key = Symbol('clipper-navigation');

export function provideViewNavigation(
	elements: () => { panel: HTMLElement; view: HTMLElement } | undefined
) {
	let revision = 0;
	let outgoing: HTMLElement | undefined;
	let animations: Animation[] = [];

	function reset() {
		revision += 1;
		for (const animation of animations) animation.cancel();
		animations = [];
		outgoing?.remove();
		outgoing = undefined;
	}

	// Adapted from https://transitions.dev/detail.html?t=page-side-by-side.
	// An inert snapshot lets the old view leave without remounting live forms.
	const navigate: Navigate = async (change, direction = 1) => {
		reset();
		const current = revision;
		const nodes = elements();
		if (!nodes || window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
			change();
			await tick();
			if (current === revision && nodes?.view.isConnected)
				nodes.view.focus({ preventScroll: true });
			return;
		}
		const { panel, view } = nodes;
		const previousHeight = panel.getBoundingClientRect().height;
		const previousViewHeight = view.getBoundingClientRect().height;
		const previous = view.cloneNode(true) as HTMLElement;
		const scrollPositions = Array.from(view.querySelectorAll<HTMLElement>('*')).map((node) => [
			node.scrollTop,
			node.scrollLeft
		]);
		previous.inert = true;
		previous.setAttribute('aria-hidden', 'true');
		Object.assign(previous.style, {
			position: 'absolute',
			top: '0',
			left: '0',
			width: '100%',
			height: `${previousViewHeight}px`,
			pointerEvents: 'none'
		});

		change();
		await tick();
		if (current !== revision || !view.isConnected) return;
		const nextHeight = panel.getBoundingClientRect().height;
		outgoing = previous;
		view.parentElement!.append(previous);
		previous.querySelectorAll<HTMLElement>('*').forEach((node, i) => {
			[node.scrollTop, node.scrollLeft] = scrollPositions[i];
		});

		const options = { duration: 250, easing: 'cubic-bezier(0.22, 1, 0.36, 1)' };
		animations = [
			previous.animate(
				[
					{ opacity: 1, transform: 'translateX(0)', filter: 'blur(0)' },
					{ opacity: 0, transform: `translateX(${-direction * 8}px)`, filter: 'blur(3px)' }
				],
				options
			),
			view.animate(
				[
					{ opacity: 0, transform: `translateX(${direction * 8}px)`, filter: 'blur(3px)' },
					{ opacity: 1, transform: 'translateX(0)', filter: 'blur(0)' }
				],
				options
			)
		];
		if (previousHeight !== nextHeight) {
			animations.push(
				panel.animate([{ height: `${previousHeight}px` }, { height: `${nextHeight}px` }], options)
			);
		}
		view.focus({ preventScroll: true });
		await Promise.allSettled(animations.map((animation) => animation.finished));
		if (current === revision) reset();
	};

	setContext(key, navigate);
	onDestroy(reset);
	return { navigate, reset };
}

export function useViewNavigation(): Navigate {
	return getContext(key);
}

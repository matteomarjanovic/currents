export function scrollFade(node: HTMLElement, enabled = true) {
	let frame = 0;
	const sync = () => {
		if (!enabled) {
			node.style.maskImage = 'none';
			return;
		}
		const top = node.scrollTop > 1;
		const bottom = node.scrollTop + node.clientHeight < node.scrollHeight - 1;
		node.style.maskImage = `linear-gradient(to bottom, ${top ? 'transparent 0, black 16px' : 'black 0'}, ${bottom ? 'black calc(100% - 16px), transparent 100%' : 'black 100%'})`;
	};
	const schedule = () => {
		cancelAnimationFrame(frame);
		frame = requestAnimationFrame(sync);
	};
	const resize = new ResizeObserver(schedule);
	const mutations = new MutationObserver(schedule);
	resize.observe(node);
	mutations.observe(node, { childList: true, subtree: true });
	node.addEventListener('scroll', sync);
	schedule();
	return {
		update(nextEnabled: boolean) {
			enabled = nextEnabled;
			sync();
		},
		destroy() {
			cancelAnimationFrame(frame);
			resize.disconnect();
			mutations.disconnect();
			node.removeEventListener('scroll', sync);
		}
	};
}

const DURATION_MS = 400;

interface LongPressOptions {
	enabled?: boolean;
	onLongPress: () => void;
}

// Fires `onLongPress` after a touch/pen hold; skips mouse (desktop already has
// hover + right-click). Cancels on any movement or early release — mirrors the
// gesture bits-ui's ContextMenuTrigger uses for organize mode's long-press-to-menu,
// so the feel matches across the app.
export function longpress(node: HTMLElement, options: LongPressOptions) {
	let timer: ReturnType<typeof setTimeout> | null = null;
	let pointerType = '';

	function clear() {
		if (timer !== null) {
			clearTimeout(timer);
			timer = null;
		}
	}

	function onPointerDown(e: PointerEvent) {
		pointerType = e.pointerType;
		if (!options.enabled || e.pointerType === 'mouse') return;
		clear();
		timer = setTimeout(() => options.onLongPress(), DURATION_MS);
	}

	function onContextMenu(e: Event) {
		// Mouse right-click belongs to the desktop context menu wrapped around the
		// card. A touch/pen contextmenu is another way browsers signal a long press.
		if (!options.enabled || pointerType === 'mouse') return;
		// Some devices/browsers signal the long-press via `contextmenu` instead of
		// (or before) our own timer completing — fire from here too, matching
		// bits-ui's ContextMenuTrigger. Clearing the timer first keeps this from
		// double-firing on top of a timer that was about to land; the drawer open
		// itself is idempotent either way.
		clear();
		options.onLongPress();
		e.preventDefault();
		e.stopPropagation();
	}

	node.addEventListener('pointerdown', onPointerDown);
	node.addEventListener('pointerup', clear);
	node.addEventListener('pointercancel', clear);
	node.addEventListener('pointermove', clear);
	node.addEventListener('contextmenu', onContextMenu, true);

	return {
		update(newOptions: LongPressOptions) {
			options = newOptions;
		},
		destroy() {
			clear();
			node.removeEventListener('pointerdown', onPointerDown);
			node.removeEventListener('pointerup', clear);
			node.removeEventListener('pointercancel', clear);
			node.removeEventListener('pointermove', clear);
			node.removeEventListener('contextmenu', onContextMenu, true);
		}
	};
}

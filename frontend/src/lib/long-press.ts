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
	let touchPress = false;

	function clear() {
		touchPress = false;
		if (timer !== null) {
			clearTimeout(timer);
			timer = null;
		}
	}

	function onPointerDown(e: PointerEvent) {
		clear();
		if (!options.enabled || (e.pointerType !== 'touch' && e.pointerType !== 'pen')) return;
		touchPress = true;
		timer = setTimeout(() => options.onLongPress(), DURATION_MS);
	}

	function onContextMenu(e: Event) {
		const type = e instanceof PointerEvent ? e.pointerType : '';
		// A menu dismissal layer can consume pointerdown. An unpaired mouse
		// contextmenu must still reach the desktop menu, never the touch drawer.
		if (!options.enabled || (type ? type !== 'touch' && type !== 'pen' : !touchPress)) return;
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
			if (!options.enabled) clear();
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

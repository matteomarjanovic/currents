// Horizontal swipe on touch. Not a general gesture kit — it exists to sit on top of a
// vertically scrolling page without stealing that scroll.

// The OS owns the screen edges: iOS's edge-swipe-back (enabled in the native shell)
// and Android's back gesture both start there and both win. Ignoring touches that
// begin in the gutter is what keeps a right-going swipe usable at all.
const EDGE_GUARD_PX = 28;
// How far a finger travels before the gesture commits to an axis.
const AXIS_LOCK_PX = 10;
// A slow drag has to cross a fraction of the element; a flick counts on distance alone.
const COMMIT_FRACTION = 0.18;
const COMMIT_MAX_PX = 90;
const FLICK_MS = 300;
const FLICK_PX = 40;

export interface SwipeOptions {
	enabled?: boolean;
	/** Live horizontal offset while the finger is down, for following the drag. */
	onMove?: (dx: number) => void;
	/** 1 when the finger went left (→ next), -1 when it went right (→ previous). */
	onSwipe: (direction: 1 | -1) => void;
	/** Ended without committing — put back whatever `onMove` moved. */
	onCancel?: () => void;
}

export function swipe(node: HTMLElement, options: SwipeOptions) {
	let startX = 0;
	let startY = 0;
	let startedAt = 0;
	let axis: 'undecided' | 'x' = 'undecided';
	let tracking = false;

	function stop(cancelled: boolean) {
		tracking = false;
		axis = 'undecided';
		if (cancelled) options.onCancel?.();
	}

	function onTouchStart(e: TouchEvent) {
		if (tracking) stop(true);
		if (!options.enabled || e.touches.length !== 1) return;
		const t = e.touches[0];
		if (t.clientX < EDGE_GUARD_PX || t.clientX > window.innerWidth - EDGE_GUARD_PX) return;
		startX = t.clientX;
		startY = t.clientY;
		startedAt = e.timeStamp;
		tracking = true;
	}

	function onTouchMove(e: TouchEvent) {
		if (!tracking) return;
		// A second finger means a pinch-zoom, which is the browser's gesture, not ours.
		if (e.touches.length !== 1) return stop(true);
		const dx = e.touches[0].clientX - startX;
		const dy = e.touches[0].clientY - startY;
		if (axis === 'undecided') {
			if (Math.abs(dx) < AXIS_LOCK_PX && Math.abs(dy) < AXIS_LOCK_PX) return;
			// The first decisive movement owns the rest of the gesture, so a drag that
			// starts vertical goes on scrolling down to the related images.
			if (Math.abs(dy) >= Math.abs(dx)) return stop(true);
			axis = 'x';
		}
		// Listener is non-passive: claiming the gesture also stops the page scrolling with it.
		e.preventDefault();
		options.onMove?.(dx);
	}

	function onTouchEnd(e: TouchEvent) {
		if (!tracking) return;
		if (axis !== 'x') return stop(true);
		const dx = (e.changedTouches[0]?.clientX ?? startX) - startX;
		const flicked = e.timeStamp - startedAt < FLICK_MS && Math.abs(dx) >= FLICK_PX;
		const dragged = Math.abs(dx) >= Math.min(node.clientWidth * COMMIT_FRACTION, COMMIT_MAX_PX);
		stop(!(flicked || dragged));
		if (flicked || dragged) options.onSwipe(dx < 0 ? 1 : -1);
	}

	function onTouchCancel() {
		if (tracking) stop(true);
	}

	node.addEventListener('touchstart', onTouchStart, { passive: true });
	node.addEventListener('touchmove', onTouchMove, { passive: false });
	node.addEventListener('touchend', onTouchEnd);
	node.addEventListener('touchcancel', onTouchCancel);

	return {
		update(newOptions: SwipeOptions) {
			options = newOptions;
		},
		destroy() {
			node.removeEventListener('touchstart', onTouchStart);
			node.removeEventListener('touchmove', onTouchMove);
			node.removeEventListener('touchend', onTouchEnd);
			node.removeEventListener('touchcancel', onTouchCancel);
		}
	};
}

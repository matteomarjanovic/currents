// Vaul's pointer gesture is cancelled once a browser commits a touch to a native
// scroller. This action follows Shards UI's arbitration model: a gesture that
// starts in a scrolled region stays list-owned, while a new downward gesture at
// the top edge owns the drawer through a non-passive document listener.

const AXIS_LOCK_PX = 6;
const CLOSE_FRACTION = 0.25;
const CLOSE_VELOCITY = 0.4; // px/ms, matching Vaul's flick threshold.
const TRANSITION = '0.5s cubic-bezier(0.32, 0.72, 0, 1)';

export function drawerScrollSwipe(node: HTMLElement, onClose: () => void) {
	let close = onClose;
	let drawer: HTMLElement | null = null;
	let overlay: HTMLElement | null = null;
	let startX = 0;
	let startY = 0;
	let lastY = 0;
	let claimY = 0;
	let dragY = 0;
	let listOwned = false;
	let axis: 'undecided' | 'vertical' | 'horizontal' = 'undecided';
	let claimed = false;
	let tracking = false;
	let samples: { y: number; time: number }[] = [];
	let resetTimer: ReturnType<typeof setTimeout> | null = null;

	const doc = node.ownerDocument;

	function resetStyles() {
		if (resetTimer !== null) {
			clearTimeout(resetTimer);
			resetTimer = null;
		}
		drawer?.style.removeProperty('transform');
		drawer?.style.removeProperty('transition');
		overlay?.style.removeProperty('opacity');
		overlay?.style.removeProperty('transition');
	}

	function stopTracking() {
		tracking = false;
	}

	function onTouchStart(event: TouchEvent) {
		if (event.touches.length !== 1) return;
		stopTracking();
		resetStyles();
		drawer = node.closest<HTMLElement>('[data-vaul-drawer]');
		if (!drawer) return;
		const openOverlays = doc.querySelectorAll<HTMLElement>(
			'[data-vaul-overlay][data-state="open"]'
		);
		overlay = openOverlays.item(openOverlays.length - 1);

		const touch = event.touches[0];
		startX = touch.clientX;
		startY = touch.clientY;
		lastY = touch.clientY;
		claimY = touch.clientY;
		dragY = 0;
		listOwned = node.scrollTop > 0;
		axis = 'undecided';
		claimed = false;
		tracking = true;
		samples = [];
	}

	function onTouchMove(event: TouchEvent) {
		const touch = event.touches[0];
		if (!tracking || !touch || !drawer || event.touches.length !== 1) return;

		const totalX = touch.clientX - startX;
		const totalY = touch.clientY - startY;
		const stepY = touch.clientY - lastY;
		lastY = touch.clientY;

		if (axis === 'undecided') {
			if (Math.abs(totalX) < AXIS_LOCK_PX && Math.abs(totalY) < AXIS_LOCK_PX) return;
			axis = Math.abs(totalY) >= Math.abs(totalX) ? 'vertical' : 'horizontal';
		}
		if (axis === 'horizontal') return;

		if (!claimed) {
			// Once a gesture scrolls the list, it stays list-owned until release,
			// even if it began at the top and returns there after reversing direction.
			if (totalY < 0 || node.scrollTop > 0) listOwned = true;
			if (listOwned || stepY <= 0) return;
			claimed = true;
			claimY = startY;
		}

		if (event.cancelable) event.preventDefault();
		event.stopPropagation();
		dragY = Math.max(0, touch.clientY - claimY);
		samples.push({ y: touch.clientY, time: event.timeStamp });
		while (samples.length > 2 && event.timeStamp - samples[0].time > 100) samples.shift();

		drawer.style.transition = 'none';
		drawer.style.transform = `translate3d(0, ${dragY}px, 0)`;
		if (overlay) {
			overlay.style.transition = 'none';
			overlay.style.opacity = `${Math.max(0, 1 - dragY / drawer.offsetHeight)}`;
		}
	}

	function velocity(): number {
		if (samples.length < 2) return 0;
		const first = samples[0];
		const last = samples[samples.length - 1];
		return last.time === first.time ? 0 : (last.y - first.y) / (last.time - first.time);
	}

	function settle(cancelled: boolean) {
		stopTracking();
		if (!claimed || !drawer) return;
		const shouldClose =
			!cancelled && (velocity() >= CLOSE_VELOCITY || dragY >= drawer.offsetHeight * CLOSE_FRACTION);
		if (shouldClose) {
			close();
			// The portal survives for Vaul's exit animation; clear our inline start
			// position after it finishes so the next opening starts cleanly.
			resetTimer = setTimeout(resetStyles, 520);
			return;
		}

		drawer.style.transition = `transform ${TRANSITION}`;
		drawer.style.transform = 'translate3d(0, 0, 0)';
		if (overlay) {
			overlay.style.transition = `opacity ${TRANSITION}`;
			overlay.style.opacity = '1';
		}
		resetTimer = setTimeout(resetStyles, 520);
	}

	function onTouchEnd() {
		settle(false);
	}

	function onTouchCancel() {
		settle(true);
	}

	// Vaul still owns gestures from the header and handle. Gestures beginning in
	// this scroller use the touch arbitration above instead of its pointer path.
	node.setAttribute('data-vaul-no-drag', '');
	doc.addEventListener('touchmove', onTouchMove, { passive: false, capture: true });
	doc.addEventListener('touchend', onTouchEnd, { capture: true });
	doc.addEventListener('touchcancel', onTouchCancel, { capture: true });
	node.addEventListener('touchstart', onTouchStart, { passive: true });
	return {
		update(next: () => void) {
			close = next;
		},
		destroy() {
			stopTracking();
			resetStyles();
			doc.removeEventListener('touchmove', onTouchMove, true);
			doc.removeEventListener('touchend', onTouchEnd, true);
			doc.removeEventListener('touchcancel', onTouchCancel, true);
			node.removeAttribute('data-vaul-no-drag');
			node.removeEventListener('touchstart', onTouchStart);
		}
	};
}

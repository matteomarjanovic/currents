// Locks background scrolling while a full-screen overlay (e.g. the save-detail) is open.
//
// The lock lives in a body CLASS (`.scroll-locked` in layout.css), not inline body styles,
// on purpose: vaul-svelte's drawer snapshots `document.body.style.cssText` when it opens and
// restores it ~300ms after it closes. If we locked via inline styles, a drawer opened over the
// overlay would capture the locked cssText and re-apply it after closing — leaving the page
// stuck unscrollable. A class isn't part of cssText, so it's immune to that restore.
export function bodyScrollY(): number {
	return document.body.classList.contains('scroll-locked')
		? -parseFloat(document.body.style.getPropertyValue('--scroll-lock-top'))
		: window.scrollY;
}

export function scrollBodyTo(y: number) {
	if (document.body.classList.contains('scroll-locked')) {
		document.body.style.setProperty('--scroll-lock-top', `${-y}px`);
	} else window.scrollTo(0, y);
}

export function lockBodyScroll(): () => void {
	const y = window.scrollY;
	document.body.style.setProperty('--scroll-lock-top', `-${y}px`);
	document.body.classList.add('scroll-locked');
	return () => {
		const restoredY = bodyScrollY();
		document.body.classList.remove('scroll-locked');
		document.body.style.removeProperty('--scroll-lock-top');
		window.scrollTo(0, restoredY);
	};
}

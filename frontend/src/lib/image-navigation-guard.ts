let blockedUntil = 0;

// Floating controls can receive a touch while Android later delivers its
// compatibility click to the image beneath them. Keep that one stray click from
// navigating; normal image taps are never delayed by more than this gesture's tail.
export function blockImageNavigation() {
	blockedUntil = Date.now() + 500;
}

export function isImageNavigationBlocked() {
	return Date.now() < blockedUntil;
}

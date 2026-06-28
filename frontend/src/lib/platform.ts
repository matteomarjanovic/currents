import { Capacitor } from '@capacitor/core';

export function isNative(): boolean {
	return Capacitor.isNativePlatform();
}

export function isIos(): boolean {
	return Capacitor.getPlatform() === 'ios';
}

export function isAndroid(): boolean {
	return Capacitor.getPlatform() === 'android';
}

export function platform(): 'ios' | 'android' | 'web' {
	return Capacitor.getPlatform() as 'ios' | 'android' | 'web';
}

// Web (non-Capacitor) platform helpers, used to decide PWA-install affordances in the
// browser. `isIos()`/`isAndroid()` above are for the native shell; these read the browser.

export function isIosWeb(): boolean {
	if (typeof navigator === 'undefined') return false;
	const ua = navigator.userAgent;
	// iPadOS 13+ Safari reports a desktop "Macintosh" UA, told apart by touch support.
	return /iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && navigator.maxTouchPoints > 1);
}

export function isMobileWeb(): boolean {
	if (typeof navigator === 'undefined') return false;
	const uaData = (navigator as Navigator & { userAgentData?: { mobile?: boolean } }).userAgentData;
	if (typeof uaData?.mobile === 'boolean') return uaData.mobile;
	return /Android/.test(navigator.userAgent) || isIosWeb();
}

export function isStandalonePwa(): boolean {
	if (typeof window === 'undefined') return false;
	return (
		window.matchMedia?.('(display-mode: standalone)').matches === true ||
		// iOS Safari doesn't support the standalone display-mode query; it sets this instead.
		(navigator as Navigator & { standalone?: boolean }).standalone === true
	);
}

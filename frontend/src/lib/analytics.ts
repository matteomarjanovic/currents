import { browser } from '$app/environment';
import { PUBLIC_UMAMI_SCRIPT_URL, PUBLIC_UMAMI_WEBSITE_ID } from '$env/static/public';
import { isNative, platform } from '$lib/platform';
import {
	sanitizeAnalyticsReferrer,
	sanitizeAnalyticsUrl,
	shouldTrackAnalyticsPath
} from '$lib/analytics-path';

export type AuthIntent = 'login' | 'signup';
export type AuthProvider = 'bluesky' | 'eurosky' | 'blacksky' | 'custom';
export type AnalyticsSurface = 'web' | 'native' | 'extension';
export type SupporterFeature = 'library_search' | 'similar_library' | 'color_search';
export type CheckoutPlacement = 'paywall' | 'settings' | 'support_page';

type EventData = {
	landing_cta_clicked: { destination: 'explore' };
	auth_started: { intent: AuthIntent; provider: AuthProvider; surface: AnalyticsSurface };
	auth_succeeded: { intent: AuthIntent; provider: AuthProvider; surface: AnalyticsSurface };
	paywall_viewed: { feature: SupporterFeature; placement: 'feature_gate' };
	checkout_started: {
		plan: 'monthly' | 'yearly';
		placement: CheckoutPlacement;
		feature?: SupporterFeature;
	};
	checkout_succeeded: {
		plan: 'monthly' | 'yearly';
		placement: CheckoutPlacement;
		feature?: SupporterFeature;
	};
};

type Umami = {
	track: (name: string, data?: Record<string, unknown>) => void;
	identify?: (data: Record<string, unknown>) => void;
};
type PendingEvent = { name: keyof EventData; data: EventData[keyof EventData] };

declare global {
	interface Window {
		umami?: Umami;
		currentsAnalyticsBeforeSend?: (
			type: string,
			payload: Record<string, unknown>
		) => Record<string, unknown> | false;
	}
}

const AUTH_ATTEMPT_KEY = 'currents-analytics-auth-attempt';
const AUTH_ATTEMPT_MAX_AGE = 60 * 60 * 1000;
const pending: PendingEvent[] = [];
let initialized = false;

function enabledHere(): boolean {
	if (!browser || !PUBLIC_UMAMI_SCRIPT_URL || !PUBLIC_UMAMI_WEBSITE_ID) return false;
	return isNative() || window.location.hostname === 'currents.is';
}

function flush(): void {
	if (!window.umami) return;
	// Session data only: no Distinct ID is supplied, so this enables anonymous
	// web/iOS/Android segmentation without joining activity to an account.
	try {
		window.umami.identify?.({ platform: platform() });
	} catch {
		// Analytics must never affect the product flow.
	}
	for (const event of pending.splice(0)) {
		try {
			window.umami.track(event.name, event.data);
		} catch {
			// Best-effort by design.
		}
	}
}

export function initAnalytics(): void {
	if (initialized || !enabledHere()) return;
	initialized = true;

	window.currentsAnalyticsBeforeSend = (_type, payload) => {
		const rawUrl = typeof payload.url === 'string' ? payload.url : window.location.href;
		if (!shouldTrackAnalyticsPath(rawUrl, window.location.href)) return false;
		return {
			...payload,
			// Several route titles contain a search query, profile name, or handle.
			// The normalized URL is enough for analytics; never send document titles.
			title: '',
			url: sanitizeAnalyticsUrl(rawUrl, window.location.href),
			referrer: sanitizeAnalyticsReferrer(
				typeof payload.referrer === 'string' ? payload.referrer : '',
				window.location.href
			)
		};
	};

	const script = document.createElement('script');
	script.defer = true;
	script.src = PUBLIC_UMAMI_SCRIPT_URL;
	script.dataset.websiteId = PUBLIC_UMAMI_WEBSITE_ID;
	script.dataset.beforeSend = 'currentsAnalyticsBeforeSend';
	script.dataset.excludeHash = 'true';
	script.dataset.doNotTrack = 'true';
	if (!isNative()) script.dataset.domains = 'currents.is';
	script.addEventListener('load', flush, { once: true });
	document.head.append(script);
}

export function trackEvent<K extends keyof EventData>(name: K, data: EventData[K]): void {
	if (!enabledHere()) return;
	if (!window.umami) {
		pending.push({ name, data });
		return;
	}
	try {
		window.umami.track(name, data);
	} catch {
		// Best-effort by design.
	}
}

export function authProvider(username: string): AuthProvider {
	if (username === 'https://bsky.social') return 'bluesky';
	if (username === 'https://eurosky.social') return 'eurosky';
	if (username === 'https://blacksky.app') return 'blacksky';
	return 'custom';
}

export function trackAuthStarted(
	intent: AuthIntent,
	provider: AuthProvider,
	surface: AnalyticsSurface
): void {
	if (browser) {
		try {
			sessionStorage.setItem(
				AUTH_ATTEMPT_KEY,
				JSON.stringify({ intent, provider, surface, startedAt: Date.now() })
			);
		} catch {
			// Storage can be unavailable in a locked-down browser; login still proceeds.
		}
	}
	trackEvent('auth_started', { intent, provider, surface });
}

// OAuth leaves Currents and then returns to a fresh app instance. sessionStorage
// links that return to the anonymous attempt in this tab without identifying the user.
export function trackPendingAuthSuccess(): void {
	if (!browser) return;
	try {
		const raw = sessionStorage.getItem(AUTH_ATTEMPT_KEY);
		if (!raw) return;
		sessionStorage.removeItem(AUTH_ATTEMPT_KEY);
		const attempt = JSON.parse(raw) as {
			intent?: AuthIntent;
			provider?: AuthProvider;
			surface?: AnalyticsSurface;
			startedAt?: number;
		};
		if (
			!attempt.intent ||
			!attempt.provider ||
			!attempt.surface ||
			!attempt.startedAt ||
			Date.now() - attempt.startedAt > AUTH_ATTEMPT_MAX_AGE
		)
			return;
		trackEvent('auth_succeeded', {
			intent: attempt.intent,
			provider: attempt.provider,
			surface: attempt.surface
		});
	} catch {
		// Missing storage or a malformed value must not affect auth itself.
	}
}

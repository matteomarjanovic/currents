import { describe, expect, it } from 'vitest';
import {
	sanitizeAnalyticsReferrer,
	sanitizeAnalyticsUrl,
	shouldTrackAnalyticsPath
} from './analytics-path';

describe('sanitizeAnalyticsUrl', () => {
	it('keeps campaign attribution and removes every other query parameter', () => {
		expect(
			sanitizeAnalyticsUrl(
				'/organize?c=at%3A%2F%2Fdid%3Aplc%3Atest%2Fcollection%2Fone&utm_source=reddit&utm_campaign=launch&q=private'
			)
		).toBe('/organize?utm_source=reddit&utm_campaign=launch');
	});

	it('redacts search text and dynamic content identifiers', () => {
		expect(sanitizeAnalyticsUrl('/search/saves/red%20dress?q=another')).toBe(
			'/search/saves/:query'
		);
		expect(sanitizeAnalyticsUrl('/profile/alice.example/save/3abc')).toBe(
			'/profile/:actor/save/:save'
		);
		expect(sanitizeAnalyticsUrl('/profile/alice.example/collection/3abc')).toBe(
			'/profile/:actor/collection/:collection'
		);
	});
});

describe('sanitizeAnalyticsReferrer', () => {
	it('sanitizes internal paths and reduces external referrers to their origin', () => {
		expect(
			sanitizeAnalyticsReferrer(
				'https://currents.is/search/saves/private?q=secret',
				'https://currents.is'
			)
		).toBe('/search/saves/:query');
		expect(
			sanitizeAnalyticsReferrer(
				'https://example.com/a/private/path?q=secret',
				'https://currents.is'
			)
		).toBe('https://example.com');
		expect(
			sanitizeAnalyticsReferrer(
				'capacitor://localhost/search/saves/private?q=secret',
				'capacitor://localhost/explore/general'
			)
		).toBe('/search/saves/:query');
	});
});

describe('shouldTrackAnalyticsPath', () => {
	it('excludes staff-only routes', () => {
		expect(shouldTrackAnalyticsPath('/admin/stats')).toBe(false);
		expect(shouldTrackAnalyticsPath('/moderation/queue/123')).toBe(false);
		expect(shouldTrackAnalyticsPath('/explore/general')).toBe(true);
	});
});

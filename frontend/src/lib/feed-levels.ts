// Discovery feed levels. The personalization dimension lives in the route
// (/explore/<slug>) rather than a query param; `value` is what getFeed expects
// as `personalized`. Anything other than General (value 0) needs a logged-in
// viewer to rank against.
export const FEED_LEVELS = [
	{ slug: 'personal', value: 1, label: 'Personal', noiseIntensity: 0.5 },
	{ slug: 'general', value: 0, label: 'General', noiseIntensity: 3 },
	{ slug: 'new-worlds', value: -1, label: 'New worlds', noiseIntensity: 7 }
] as const;

export type FeedLevel = (typeof FEED_LEVELS)[number];

export const findFeedLevel = (slug: string | undefined): FeedLevel | undefined =>
	FEED_LEVELS.find((l) => l.slug === slug);

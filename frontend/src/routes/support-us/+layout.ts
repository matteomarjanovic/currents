// Prerendered for SEO, unlike the rest of the app (see the root `+layout.ts`) — it's a public
// marketing/paywall page and App Store rules mean native apps never show purchase buttons here
// anyway, so there's nothing native-only to lose by rendering it statically.
export const ssr = true;
export const prerender = true;

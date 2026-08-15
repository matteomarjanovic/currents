// Static marketing/paywall content stays prerendered. App Store rules mean native apps never show
// purchase buttons here anyway, so there is nothing native-only to lose.
export const ssr = true;
export const prerender = true;

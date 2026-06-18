// Shown when a save / upload / resave is rejected with HTTP 429 because the
// user's PDS is rate-limiting blob uploads. The appview uploads on the user's
// behalf, so a busy PDS can throttle even light users. The throttle is per-PDS
// and transient, so the copy reassures rather than blames. Kept identical to
// the backend copy (appview/records.go `rateLimitMessage`).
export const RATE_LIMIT_MESSAGE =
	'Your data server is temporarily limiting uploads. Please try again in a few minutes.';

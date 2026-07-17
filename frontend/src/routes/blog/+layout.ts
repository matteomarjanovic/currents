// The rest of the app is a client-only SPA (see the root `+layout.ts`) so it can be bundled
// into the Capacitor mobile apps. The blog is prerendered instead, for SEO: it's plain marketing
// content with no auth-gated data, so static HTML serves it better than a client-rendered shell.
export const ssr = true;
export const prerender = true;

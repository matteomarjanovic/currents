// The blog is static marketing content, so prerender it even though the normal web build now has
// a runtime SSR server. The Capacitor build still disables SSR at the root.
export const ssr = true;
export const prerender = true;

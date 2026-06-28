import postcssOklabFunction from '@csstools/postcss-oklab-function';
import postcssProgressiveCustomProperties from '@csstools/postcss-progressive-custom-properties';

// Tailwind v4 emits oklch() throughout (its whole palette + our theme tokens), which
// Chromium <111 (e.g. Brave from 2022) can't parse — it drops the value and the UI loses
// its colors. These plugins ADD an sRGB fallback ahead of each oklch() while preserving the
// original (`preserve: true`), so modern browsers render the exact same oklch() as before and
// only old browsers use the fallback. progressive-custom-properties wraps the preserved
// custom-property values in @supports (required for the `--token: oklch(...)` case).
export default {
	plugins: [postcssOklabFunction({ preserve: true }), postcssProgressiveCustomProperties()]
};

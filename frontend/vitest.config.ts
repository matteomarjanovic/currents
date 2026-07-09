import { defineConfig } from 'vitest/config';

// Unit tests for pure TypeScript modules (no component rendering, no SvelteKit
// runtime) — component/flow coverage lives in the Playwright e2e suite.
export default defineConfig({
	test: {
		include: ['src/**/*.test.ts'],
		environment: 'node'
	}
});

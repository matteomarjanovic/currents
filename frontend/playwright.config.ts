import { defineConfig, devices } from '@playwright/test';

// E2E tests run against the dev server on a mobile (touch) viewport. The appview API is
// mocked per-test, so no backend or login is required.
export default defineConfig({
	testDir: './e2e',
	timeout: 60_000,
	fullyParallel: false,
	use: { baseURL: 'http://localhost:5173' },
	projects: [{ name: 'mobile-chromium', use: { ...devices['Pixel 5'] } }],
	webServer: {
		command: 'npm run dev -- --port 5173',
		url: 'http://localhost:5173',
		reuseExistingServer: !process.env.CI,
		timeout: 120_000
	}
});

import { expect, test } from '@playwright/test';

test.beforeEach(async ({ page }) => {
	await page.route('https://api-dev.currents.is/**', (route) =>
		route.fulfill({ status: 401, body: 'not authenticated' })
	);
});

test('the public landing page previews modes, colors and feeds', async ({ page }) => {
	await page.emulateMedia({ colorScheme: 'light', reducedMotion: 'reduce' });
	await page.goto('/?utm_source=landing-test');
	await expect(page).toHaveTitle('Currents | The place for your visual inspiration');
	await expect(page.locator('link[rel="canonical"]')).toHaveAttribute(
		'href',
		'https://currents.is/'
	);
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Get carried by the currents.');
	await expect(page.locator('html[data-theme]')).toBeAttached();
	await expect(page.locator('html')).toHaveCSS('color-scheme', 'dark');
	await expect(page.getByRole('button', { name: 'Toggle theme' })).toHaveCount(0);
	const banner = page.getByRole('banner');
	await expect(banner).toHaveCSS('position', 'fixed');
	expect(await banner.evaluate((el) => getComputedStyle(el, '::after').opacity)).toBe('1');

	const explore = page.getByRole('tab', { name: 'Explore' });
	await expect(explore).toHaveAttribute('aria-selected', 'true');
	await explore.focus();
	await page.keyboard.press('ArrowRight');
	await expect(page.getByRole('tab', { name: 'Organize' })).toHaveAttribute(
		'aria-selected',
		'true'
	);
	await expect(page.getByRole('heading', { name: 'Give it a place to live.' })).toBeVisible();

	const vermilion = page.getByRole('button', { name: 'Vermilion' });
	await vermilion.click();
	await expect(vermilion).toHaveAttribute('aria-pressed', 'true');
	await expect(
		page.getByRole('img', { name: 'Red metal grating crossed by a dark chain' })
	).toBeVisible();

	await page.getByRole('button', { name: 'Choose an example feed' }).click();
	await page.getByRole('menuitemradio', { name: 'New worlds' }).click();
	await expect(page.getByText('Unexplored interests near the things you save.')).toBeVisible();
	await expect.poll(() => page.evaluate(() => scrollY)).toBeGreaterThan(100);
	expect(await banner.evaluate((el) => el.getBoundingClientRect().top)).toBe(0);
	await expect(banner.getByRole('link', { name: 'Log in' })).toBeInViewport();

	const exploreLinks = page.getByRole('main').getByRole('link', { name: 'Explore Currents' });
	await expect(exploreLinks).toHaveCount(2);
	for (const link of await exploreLinks.all())
		await expect(link).toHaveAttribute('href', '/explore');
	await expect(
		page.getByRole('main').getByRole('link', { name: 'Support the project' })
	).toHaveAttribute('href', '/support-us');
});

test('landing search opens the app search and keeps the visitor’s theme preference', async ({
	page
}) => {
	await page.emulateMedia({ colorScheme: 'light' });
	await page.addInitScript(() => localStorage.setItem('mode-watcher-mode', 'light'));
	await page.goto('/');
	await expect(page.locator('html[data-theme]')).toBeAttached();
	await expect(page.locator('html')).toHaveCSS('color-scheme', 'dark');
	await expect(page.getByRole('banner').getByRole('link', { name: 'Log in' })).toHaveAttribute(
		'href',
		'/login'
	);

	await page.getByRole('banner').getByRole('button', { name: 'Search', exact: true }).click();
	const search = page.getByRole('dialog', { name: 'Search' });
	await expect(search).toBeVisible();
	await search.getByRole('combobox').fill('warm interiors');
	await search.getByText('Search images for “warm interiors”').click();

	await expect(page).toHaveURL('/search/saves/warm%20interiors');
	await expect(page.locator('html')).toHaveCSS('color-scheme', 'light');
	expect(await page.evaluate(() => localStorage.getItem('mode-watcher-mode'))).toBe('light');
});

test('search and login stay in place when a guest opens Explore', async ({ page }) => {
	const banner = page.getByRole('banner');
	const search = banner.getByRole('button', { name: 'Search', exact: true });
	const login = banner.getByRole('link', { name: 'Log in' });
	const theme = banner.getByRole('button', { name: 'Toggle theme' });
	const measures = () =>
		Promise.all(
			[search, login].map(async (control) => ({
				box: await control.boundingBox(),
				shadow: await control.evaluate((el) => getComputedStyle(el).boxShadow)
			}))
		);

	for (const width of [1440, 390]) {
		await page.setViewportSize({ width, height: 900 });
		await page.goto('/');
		await expect(banner.getByRole('link', { name: 'Currents home' })).toBeVisible();
		await page.evaluate(() => document.fonts.ready);
		const before = await measures();
		await expect(theme).toHaveCount(0);
		await page.getByRole('main').getByRole('link', { name: 'Explore Currents' }).first().click();
		await expect(page).toHaveURL('/explore/general');
		await expect(theme).toBeVisible();
		await expect.poll(measures).toEqual(before);
		const themeBox = await theme.boundingBox();
		const [searchBefore, loginBefore] = before;
		expect(searchBefore.box!.height).toBe(loginBefore.box!.height);
		expect(themeBox!.height).toBe(loginBefore.box!.height);
		expect(themeBox!.x).toBeLessThan(searchBefore.box!.x);
		expect(searchBefore.box!.x).toBeLessThan(loginBefore.box!.x);
	}
});

test('scrolling through the pinned mockup crossfades into Organize and back', async ({ page }) => {
	await page.emulateMedia({ reducedMotion: 'no-preference' });
	await page.goto('/');
	await expect(page.locator('html[data-theme]')).toBeAttached();

	const showcase = page.getByRole('region', { name: 'Explore and Organize' });
	const explore = showcase.getByRole('tab', { name: 'Explore' });
	const organize = showcase.getByRole('tab', { name: 'Organize' });
	const stage = showcase.locator('.stage');
	const progress = () =>
		stage.evaluate((el) => Number(el.style.getPropertyValue('--landing-mode-progress')));
	const scrollThrough = (fraction: number) =>
		showcase.evaluate((el, fraction) => {
			const stage = el.firstElementChild as HTMLElement;
			const distance = el.offsetHeight - stage.offsetHeight;
			const start =
				scrollY + el.getBoundingClientRect().top - parseFloat(getComputedStyle(stage).top);
			window.scrollTo(0, start + distance * fraction);
		}, fraction);

	await scrollThrough(0);
	await expect(explore).toHaveAttribute('aria-selected', 'true');
	await expect(showcase.getByRole('heading', { name: 'Discover new worlds.' })).toBeInViewport();

	await scrollThrough(0.2);
	await expect.poll(progress).toBe(0);

	await scrollThrough(0.4);
	await expect.poll(progress).toBeGreaterThan(0.25);
	expect(await progress()).toBeLessThan(0.5);
	await expect(stage).toBeInViewport({ ratio: 0.95 });

	await scrollThrough(0.8);
	await expect(organize).toHaveAttribute('aria-selected', 'true');
	await expect.poll(progress).toBe(1);
	const details = showcase.getByRole('tabpanel', { name: 'Organize' });
	await expect(details.getByText('Palette', { exact: true })).toBeVisible();
	await expect(details.getByText('Dimensions', { exact: true })).toBeVisible();
	await expect(
		showcase.getByRole('heading', { name: 'Give it a place to live.' })
	).toBeInViewport();

	await scrollThrough(0);
	await expect(explore).toHaveAttribute('aria-selected', 'true');
	await expect.poll(progress).toBe(0);
});

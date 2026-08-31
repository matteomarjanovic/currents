import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:test', handle: 'test.bsky.social', displayName: 'Tester' };
const ownCollections = [
	{
		uri: `at://${me.did}/is.currents.feed.collection/architecture`,
		author: me,
		name: 'Architecture',
		previews: [],
		saveCount: 4
	},
	{
		uri: `at://${me.did}/is.currents.feed.collection/nature`,
		author: me,
		name: 'Nature',
		previews: [],
		saveCount: 7
	}
];

type FeedPreferences = { excludedCollections: string[]; defaultFeed: string };
type UserPreferences = {
	gifAutoplay: boolean;
	organizeCollectionSort: string;
	saveSuggestionMode: string;
};

async function mockApi(page: Page, writes: FeedPreferences[], preferenceWrites: UserPreferences[]) {
	await page.route(`${APPVIEW}/**`, async (route) => {
		const url = route.request().url();
		const json = (value: unknown) =>
			route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(value) });

		if (url.includes('/api/feed/preferences')) {
			if (route.request().method() === 'PUT') {
				const body = route.request().postDataJSON() as FeedPreferences;
				writes.push(body);
				return route.fulfill({ status: 204, body: '' });
			}
			return json({ excludedCollections: [ownCollections[1].uri], defaultFeed: 'personal' });
		}
		if (url.includes('/api/me')) return json(me);
		if (url.includes('getActorCollections')) return json({ collections: ownCollections });
		if (url.includes('getFeed')) return json({ feed: [], cursor: null });
		if (url.includes('/api/supporter/status')) {
			return json({ active: true, subscribed: false, colorTrialsLeft: 5 });
		}
		if (url.includes('/api/preferences')) {
			if (route.request().method() === 'PUT') {
				preferenceWrites.push(route.request().postDataJSON() as UserPreferences);
				return route.fulfill({ status: 204, body: '' });
			}
			return json({
				gifAutoplay: true,
				organizeCollectionSort: 'name',
				saveSuggestionMode: 'recommended-then-last-used'
			});
		}
		if (url.includes('moderation/prefs')) {
			return json({
				porn: 'blur',
				sexual: 'blur',
				nudity: 'blur',
				graphicMedia: 'blur',
				aiGenerated: 'show'
			});
		}
		if (url.includes('features/seen')) return json({ seen: [] });
		return json({});
	});
}

test('Feed settings load, search, and persist excluded collections', async ({ page }) => {
	const writes: FeedPreferences[] = [];
	const preferenceWrites: UserPreferences[] = [];
	await mockApi(page, writes, preferenceWrites);
	await page.goto('/settings/feed');

	await expect(page.getByRole('heading', { name: 'Feed preferences' })).toBeVisible();
	const defaultFeed = page.getByRole('radiogroup', { name: 'Default feed' });
	await expect(defaultFeed.getByRole('radio', { name: 'Personal' })).toHaveAttribute(
		'aria-checked',
		'true'
	);
	await defaultFeed.getByRole('radio', { name: 'New worlds' }).click();
	await expect.poll(() => writes.at(-1)?.defaultFeed).toBe('new-worlds');
	const quickSave = page.getByRole('radiogroup', { name: 'Quick Save destination' });
	await expect(
		quickSave.getByRole('radio', { name: /Match, then follow my choice/ })
	).toBeChecked();
	await quickSave.getByRole('radio', { name: /Match every image/ }).click();
	await expect.poll(() => preferenceWrites.at(-1)?.saveSuggestionMode).toBe('recommended');
	const mobileNav = page.getByTestId('settings-mobile-nav');
	await expect(mobileNav).toBeVisible();
	expect(
		await mobileNav.evaluate((nav) => {
			const bounds = nav.getBoundingClientRect();
			const parentBounds = nav.parentElement!.getBoundingClientRect();
			return bounds.left >= parentBounds.left && bounds.right <= parentBounds.right;
		})
	).toBe(true);
	const combobox = page.getByRole('combobox', {
		name: 'Choose collections to exclude from feed personalization'
	});
	await expect(combobox).toContainText('1 collection excluded');

	await combobox.click();
	const search = page.getByPlaceholder('Search collections…');
	await search.fill('Architecture');
	await expect(page.getByText('Architecture', { exact: true })).toBeVisible();
	await expect(page.getByText('Nature', { exact: true })).toBeHidden();

	await page.getByText('Architecture', { exact: true }).click();
	await expect(combobox).toContainText('2 collections excluded');
	await expect
		.poll(() => writes.at(-1)?.excludedCollections)
		.toEqual([ownCollections[1].uri, ownCollections[0].uri]);

	await search.fill('Nature');
	await page.getByText('Nature', { exact: true }).click();
	await expect(combobox).toContainText('1 collection excluded');
	await expect.poll(() => writes.at(-1)?.excludedCollections).toEqual([ownCollections[0].uri]);
});

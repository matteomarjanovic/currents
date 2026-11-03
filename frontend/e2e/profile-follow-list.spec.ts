import { test, expect, type Page } from '@playwright/test';

const APPVIEW = 'https://api-dev.currents.is';
const me = { did: 'did:plc:viewer', handle: 'viewer.test', displayName: 'Me' };
const alice = { did: 'did:plc:alice', handle: 'alice.test', displayName: 'Alice' };
const bob = {
	did: 'did:plc:bob',
	handle: 'bob.test',
	displayName: 'Bob',
	viewer: { following: `at://${me.did}/is.currents.graph.follow/bob` }
};
const profile = {
	did: 'did:plc:subject',
	handle: 'subject.test',
	displayName: 'Subject',
	followersCount: 3,
	followsCount: 3
};

async function mockApi(page: Page, loggedIn = true, followStatus = 200) {
	const calls = { followed: [] as string[], unfollowed: [] as string[] };
	await page.route(`${APPVIEW}/**`, (route) => {
		const request = route.request();
		const path = new URL(request.url()).pathname;
		const json = (value: unknown, status = 200) => route.fulfill({ status, json: value });
		if (path === '/api/me') return loggedIn ? json(me) : route.fulfill({ status: 401 });
		if (path.endsWith('getProfile')) return json(profile);
		if (path.endsWith('getFollowers')) return json({ followers: [alice, bob, me] });
		if (path.endsWith('getFollows')) return json({ follows: [alice, bob, me] });
		if (path === '/api/follow' && request.method() === 'POST') {
			calls.followed.push(request.postDataJSON().subject);
			return json(
				followStatus === 403
					? { error: 'ScopeMissing' }
					: { uri: `at://${me.did}/is.currents.graph.follow/alice` },
				followStatus
			);
		}
		if (path.startsWith('/api/follow/') && request.method() === 'DELETE') {
			calls.unfollowed.push(path.split('/').pop()!);
			return route.fulfill({ status: 204 });
		}
		if (path.includes('features/seen')) return json({ seen: [] });
		return json({ collections: [], saves: [] });
	});
	return calls;
}

for (const tab of ['Followers', 'Following']) {
	test(`${tab} rows follow and unfollow without navigating or closing the dialog`, async ({
		page
	}) => {
		const calls = await mockApi(page);
		await page.goto(`/profile/${profile.handle}`);
		await page
			.getByRole('button', {
				name: tab === 'Followers' ? '3 followers' : '3 following',
				exact: true
			})
			.click();
		const dialog = page.getByRole('dialog', { name: "Subject's connections" });
		await expect(dialog.getByRole('button', { name: 'Follow Alice', exact: true })).toBeVisible();
		await expect(dialog.getByRole('button', { name: 'Unfollow Bob', exact: true })).toHaveText(
			'Following'
		);
		await expect(dialog.getByRole('button', { name: /(?:Follow|Unfollow) Me$/ })).toHaveCount(0);
		await dialog.getByRole('button', { name: 'Follow Alice', exact: true }).click();
		await expect(dialog.getByRole('button', { name: 'Unfollow Alice', exact: true })).toHaveText(
			'Following'
		);
		expect(calls.followed).toEqual([alice.did]);
		await dialog.getByRole('button', { name: 'Unfollow Alice', exact: true }).click();
		await expect(dialog.getByRole('button', { name: 'Follow Alice', exact: true })).toBeVisible();
		await dialog.getByRole('button', { name: 'Unfollow Bob', exact: true }).click();
		await expect(dialog.getByRole('button', { name: 'Follow Bob', exact: true })).toBeVisible();
		expect(calls.unfollowed).toEqual(['alice', 'bob']);
		await expect(dialog).toBeVisible();
		await expect(page).toHaveURL(`/profile/${profile.handle}`);
		await dialog.getByRole('link', { name: /Alice/ }).click();
		await expect(page).toHaveURL(`/profile/${alice.handle}`);
		await expect(dialog).toBeHidden();
	});
}

test('anonymous visitors can browse connections without follow buttons', async ({ page }) => {
	await mockApi(page, false);
	await page.goto(`/profile/${profile.handle}`);
	await page.getByRole('button', { name: '3 followers', exact: true }).click();
	const dialog = page.getByRole('dialog', { name: "Subject's connections" });
	await expect(dialog.getByRole('link', { name: /Alice/ })).toBeVisible();
	await expect(dialog.getByRole('button', { name: /^(?:Follow|Unfollow) / })).toHaveCount(0);
});

test('following from the list handles missing OAuth permission', async ({ page }) => {
	await mockApi(page, true, 403);
	await page.goto(`/profile/${profile.handle}`);
	await page.getByRole('button', { name: '3 followers', exact: true }).click();
	await page.getByRole('button', { name: 'Follow Alice', exact: true }).click();
	await expect(page.getByRole('dialog', { name: 'New permission required' })).toBeVisible();
});

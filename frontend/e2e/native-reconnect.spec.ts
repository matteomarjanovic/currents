import { expect, test } from '@playwright/test';

const me = { did: 'did:plc:test', handle: 'test.bsky.social' };
const collectionUri = `at://${me.did}/is.currents.feed.collection/ideas`;
const save = {
	uri: `at://${me.did}/is.currents.feed.save/image`,
	author: me,
	content: {
		$type: 'is.currents.content.defs#imageView',
		blobCid: 'image-cid',
		imageUrl:
			'data:image/svg+xml,' +
			encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="400" height="500"/>'),
		width: 400,
		height: 500
	},
	createdAt: '2026-09-01T00:00:00Z',
	viewer: { saves: [] }
};

for (const platform of ['android', 'ios']) {
	test(`${platform}: reconnect returns to the mounted app and saving uses the new session`, async ({
		page
	}) => {
		// Emulate only the OS/plugin boundary. Login, secure storage, API requests,
		// callback handling and the collection selector run through the app code.
		await page.addInitScript((platform) => {
			Object.assign(window, {
				...(platform === 'android'
					? { androidBridge: {} }
					: { webkit: { messageHandlers: { bridge: {} } } })
			});
			localStorage.setItem('test-auth-token', 'old-token');
			const methods: Record<string, string[]> = {
				App: ['addListener', 'removeListener'],
				Browser: ['open', 'close'],
				NativeAuth: ['isSupported', 'open'],
				SecurePreferences: ['get', 'set', 'remove'],
				SharedAuth: ['set', 'clear'],
				SplashScreen: ['hide'],
				SafeArea: ['setStatusBarStyle', 'setNavigationBarStyle'],
				AccessibilityPreferences: ['getPreferences']
			};
			Object.assign(window, {
				Capacitor: {
					PluginHeaders: Object.entries(methods).map(([name, methods]) => ({
						name,
						methods: methods.map((name) => ({
							name,
							rtype: name === 'addListener' ? 'callback' : 'promise'
						}))
					})),
					async nativePromise(plugin: string, method: string, options: Record<string, string>) {
						if (plugin === 'SecurePreferences') {
							if (method === 'get') return { value: localStorage.getItem('test-auth-token') };
							if (method === 'set') localStorage.setItem('test-auth-token', options.value);
						}
						if (plugin === 'AccessibilityPreferences') return { fontScale: 1 };
						if (plugin === 'NativeAuth' && method === 'isSupported') return { value: true };
						if ((plugin === 'NativeAuth' || plugin === 'Browser') && method === 'open') {
							localStorage.setItem('test-login-url', options.url);
							if (plugin === 'NativeAuth') {
								return new Promise((resolve) => {
									window.addEventListener(
										'test-oauth-return',
										(event) => resolve({ url: (event as CustomEvent).detail }),
										{ once: true }
									);
								});
							}
						}
						return {};
					},
					nativeCallback(
						plugin: string,
						_method: string,
						options: { eventName: string },
						callback: (event: { url: string }) => void
					) {
						if (plugin === 'App' && options.eventName === 'appUrlOpen') {
							window.addEventListener('test-oauth-return', (event) => {
								if (platform === 'ios') callback({ url: (event as CustomEvent).detail });
							});
						}
						return 'listener';
					}
				}
			});
		}, platform);

		let saved = false;
		let profileRefreshes = 0;
		await page.route(/https?:\/\/[^/]+\/(?:xrpc|api)\//, async (route) => {
			const path = new URL(route.request().url()).pathname;
			const reconnected = route.request().headers().authorization === 'Bearer new-token';
			const json = (value: unknown, status = 200) =>
				route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(value) });
			if (path === '/api/me') {
				if (reconnected) profileRefreshes++;
				return json(me);
			}
			if (path.endsWith('getActorCollections')) {
				return json({
					collections: [
						{
							uri: collectionUri,
							author: me,
							name: reconnected ? 'Reconnected Collection' : 'Test Collection',
							previews: [],
							saveCount: 1,
							createdAt: save.createdAt
						}
					]
				});
			}
			if (path.endsWith('getFeed')) return json({ feed: [save] });
			if (path.endsWith('/resave')) {
				if (!reconnected) return json({ rateLimited: true, needsReauth: true }, 429);
				saved = true;
				return json({ uri: `at://${me.did}/is.currents.feed.save/new` });
			}
			if (path.endsWith('/features/seen')) return json({ seen: [] });
			return json({});
		});

		await page.goto('/explore/general');
		const image = page.locator('a.block').first();
		await expect(image).toBeVisible();
		const longPress = async () => {
			const box = (await image.boundingBox())!;
			const client = await page.context().newCDPSession(page);
			const point = { x: box.x + box.width / 2, y: box.y + box.height / 2 };
			await client.send('Input.dispatchTouchEvent', { type: 'touchStart', touchPoints: [point] });
			await expect(page.getByText('Quick actions', { exact: true })).toBeVisible();
			await client.send('Input.dispatchTouchEvent', { type: 'touchEnd', touchPoints: [] });
			await client.detach();
		};
		await longPress();
		await page.getByRole('button', { name: 'Quick save to Test Collection', exact: true }).click();
		await expect(page.getByRole('button', { name: 'Reconnect', exact: true })).toBeVisible();
		await page.keyboard.press('Escape');
		await expect(page.getByText('Quick actions', { exact: true })).toBeHidden();
		await page.getByRole('button', { name: 'Reconnect', exact: true }).click();
		await expect
			.poll(() => page.evaluate(() => localStorage.getItem('test-login-url')))
			.toBeTruthy();
		const loginUrl = new URL((await page.evaluate(() => localStorage.getItem('test-login-url')))!);
		const returnTo =
			platform === 'android' ? 'is.currents.app://oauth-callback' : 'currents://oauth-callback';
		expect(loginUrl.searchParams.get('return_to')).toBe(returnTo);
		expect(loginUrl.searchParams.get('username')).toBe(me.did);
		await page.evaluate((url) => {
			window.dispatchEvent(new CustomEvent('test-oauth-return', { detail: url }));
		}, `${returnTo}?token=new-token`);
		await expect.poll(() => profileRefreshes).toBe(1);
		await expect(page).toHaveURL(/\/explore\/general$/);
		await longPress();
		await page
			.getByRole('button', { name: 'Quick save to Reconnected Collection', exact: true })
			.click();
		await expect.poll(() => saved).toBe(true);
	});
}

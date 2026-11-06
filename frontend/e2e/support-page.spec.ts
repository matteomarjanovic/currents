import { test, expect } from '@playwright/test';

test.beforeEach(async ({ page }) => {
	await page.route('https://api-dev.currents.is/**', (route) =>
		route.fulfill({ status: 401, body: 'not authenticated' })
	);
});

test('support stays accessible without signing in and offers the right contacts', async ({
	page
}) => {
	await page.goto('/support');
	await page.waitForLoadState('networkidle');

	await expect(page).toHaveURL('/support');
	await expect(page).toHaveTitle('Support · Currents');
	await expect(page.getByRole('heading', { name: 'Support', exact: true })).toBeVisible();
	for (const email of ['support@currents.is', 'refunds@currents.is', 'privacy@currents.is']) {
		await expect(page.getByRole('link', { name: email, exact: true })).toHaveAttribute(
			'href',
			`mailto:${email}`
		);
	}
	await expect(
		page.getByRole('link', { name: 'our UserInput board', exact: true })
	).toHaveAttribute(
		'href',
		'https://userinput.app/s/did:plc:jaur46k6ijyfvl4lojza7eic/3mob6zfay3e2d'
	);
});

test('the footer links to support from a public page', async ({ page }) => {
	await page.goto('/privacy');
	await page.getByRole('link', { name: 'Help & support', exact: true }).click();

	await expect(page).toHaveURL('/support');
	await expect(page.getByRole('heading', { name: 'Support', exact: true })).toBeVisible();
});

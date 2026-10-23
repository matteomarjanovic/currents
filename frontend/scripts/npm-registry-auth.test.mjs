import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import test from 'node:test';

test('project registry configuration inherits user credentials', { timeout: 15000 }, async () => {
	const root = mkdtempSync(join(tmpdir(), 'currents-npm-auth-'));
	let authenticated = false;
	const server = createServer((req, res) => {
		authenticated = req.headers.authorization === 'Bearer fixture-token';
		res.setHeader('Content-Type', 'application/json');
		res.end(
			JSON.stringify({
				name: '@capawesome-team/registry-auth-fixture',
				'dist-tags': { latest: '1.0.0' },
				versions: { '1.0.0': { name: '@capawesome-team/registry-auth-fixture', version: '1.0.0' } }
			})
		);
	});
	try {
		await new Promise((resolve, reject) => {
			server.once('error', reject);
			server.listen(0, '127.0.0.1', resolve);
		});
		const host = `127.0.0.1:${server.address().port}`;
		const config = readFileSync(new URL('../.npmrc', import.meta.url), 'utf8')
			.replaceAll('npm.registry.capawesome.io', host)
			.replaceAll(`https://${host}`, `http://${host}`);
		writeFileSync(join(root, '.npmrc'), config);
		writeFileSync(join(root, 'package.json'), '{"private":true}');
		writeFileSync(join(root, 'user.npmrc'), `//${host}/:_authToken=fixture-token\n`);
		writeFileSync(join(root, 'global.npmrc'), '');
		const result = await new Promise((resolve, reject) => {
			const child = spawn(
				'npm',
				[
					'view',
					'@capawesome-team/registry-auth-fixture',
					'version',
					'--json',
					`--userconfig=${join(root, 'user.npmrc')}`,
					`--globalconfig=${join(root, 'global.npmrc')}`,
					`--cache=${join(root, 'cache')}`
				],
				{ cwd: root, env: { PATH: process.env.PATH, NO_UPDATE_NOTIFIER: '1' } }
			);
			let stderr = '';
			child.stderr.on('data', (chunk) => {
				stderr += chunk;
			});
			child.once('error', reject);
			child.once('close', (code) => resolve({ code, stderr }));
		});
		assert.equal(result.code, 0, result.stderr);
		assert.equal(
			authenticated,
			true,
			'npm must use the fixture credential from its user configuration'
		);
	} finally {
		await new Promise((resolve) => server.close(resolve));
		rmSync(root, { recursive: true, force: true });
	}
});

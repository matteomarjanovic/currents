import assert from 'node:assert/strict';
import { copyFileSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join } from 'node:path';
import { spawnSync } from 'node:child_process';
import test from 'node:test';

for (const scenario of [
	{ name: 'missing plugin', files: [], allowed: false },
	{ name: 'web-only stub', files: ['package.json', 'index.js'], allowed: false },
	{ name: 'missing Android implementation', files: ['Package.swift'], allowed: false },
	{ name: 'missing iOS implementation', files: ['android/build.gradle'], allowed: false },
	{
		name: 'native plugin installed',
		files: ['Package.swift', 'android/build.gradle'],
		allowed: true
	}
]) {
	test(scenario.name, () => {
		const root = mkdtempSync(join(tmpdir(), 'currents-native-deps-'));
		try {
			const script = join(root, 'scripts', 'check-native-dependencies.mjs');
			mkdirSync(dirname(script), { recursive: true });
			copyFileSync(new URL('./check-native-dependencies.mjs', import.meta.url), script);
			for (const file of scenario.files) {
				const path = join(root, 'node_modules/@capawesome-team/capacitor-secure-preferences', file);
				mkdirSync(dirname(path), { recursive: true });
				writeFileSync(path, 'test fixture');
			}
			const result = spawnSync(process.execPath, [script], { encoding: 'utf8' });
			assert.equal(result.status, scenario.allowed ? 0 : 1, result.stderr);
			if (!scenario.allowed) assert.match(result.stderr, /No native sync has been started/);
		} finally {
			rmSync(root, { recursive: true, force: true });
		}
	});
}

import { existsSync } from 'node:fs';

const plugin = new URL(
	'../node_modules/@capawesome-team/capacitor-secure-preferences/',
	import.meta.url
);
const required = ['Package.swift', 'android/build.gradle'];

if (required.some((file) => !existsSync(new URL(file, plugin)))) {
	console.error(
		'Native builds require the real @capawesome-team/capacitor-secure-preferences package. ' +
			'A web-only stub cannot be used. Refresh your Capawesome registry credentials, ' +
			'install the plugin, then retry. No native sync has been started.'
	);
	process.exit(1);
}

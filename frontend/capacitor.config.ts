import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
	appId: 'is.currents.app',
	appName: 'Currents',
	webDir: 'build',
	server: {
		androidScheme: 'https'
	}
};

if (process.env.CAP_DEV_URL) {
	config.server = {
		...config.server,
		url: process.env.CAP_DEV_URL,
		cleartext: true
	};
}

export default config;

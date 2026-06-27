/// <reference types="@capacitor-community/safe-area" />
import type { CapacitorConfig } from '@capacitor/cli';

const config: CapacitorConfig = {
	appId: 'is.currents.app',
	appName: 'Currents',
	webDir: 'build',
	server: {
		androidScheme: 'https'
	},
	plugins: {
		// Edge-to-edge is enforced on Android 15+, so the webview draws under the system bars.
		// This plugin makes env(safe-area-inset-*) resolve to the real insets on Android.
		SafeArea: {
			detectViewportFitCoverChanges: true,
			initialViewportFitCover: true,
			statusBarStyle: 'DEFAULT',
			navigationBarStyle: 'DEFAULT'
		},
		// Capacitor core's built-in SystemBars also handles insets by default (insetsHandling:
		// 'css'), which double-counts the keyboard against the SafeArea plugin (viewport
		// collapses to ~136px). Disable it so SafeArea is the sole inset handler.
		SystemBars: {
			insetsHandling: 'disable'
		},
		// On Android 12+ the OS owns the launch splash (circular app icon on windowSplashScreenBackground
		// — set to the brand color in styles.xml). launchAutoHide:false keeps it up until the root
		// layout fades it out once content has painted, so there's no blank flash between splash and app.
		SplashScreen: {
			launchAutoHide: false,
			launchFadeOutDuration: 200
		}
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

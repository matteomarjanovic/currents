import { clientsClaim } from '@vite-pwa/workbox-swkit/core';
import { cleanupOutdatedCaches, precacheAndRoute } from '@vite-pwa/workbox-swkit/precaching';
import '../../static/share-target-sw.js';

declare const self: ServiceWorkerGlobalScope & {
	__WB_MANIFEST: Array<{ url: string; revision: string | null }>;
};

void self.skipWaiting();
clientsClaim();
cleanupOutdatedCaches();
precacheAndRoute(self.__WB_MANIFEST);

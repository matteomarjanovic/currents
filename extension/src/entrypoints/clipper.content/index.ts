import { mount, unmount } from 'svelte';
import App from './App.svelte';
import { showClipper, loadClipperAuth, type SiteHints } from '../../lib/clipper-store.svelte';
import { bestImageUrl, candidateForImage, collectPageImages } from './collect-images';
import { detectImageOrigin } from './detect-image-origin';
import { setupGenericImageButton, setupPinterestGrid } from './image-buttons';
import { isInstagramHost, resolveInstagramImage } from './instagram-image';
import { isThreadsHost, resolveThreadsImage } from './threads-image';
import {
	isPinterestHost,
	loadSaveButtonSettings,
	showButtonOnSite
} from '../../lib/save-button-settings.svelte';
import '../../lib/theme.css';

// @font-face rules inside a shadow root are ignored, so the font is declared
// at the document level the first time the clipper is shown.
let fontInjected = false;
function injectFont() {
	if (fontInjected) return;
	fontInjected = true;
	const style = document.createElement('style');
	style.textContent = `@font-face{font-family:'Instrument Sans Variable';font-style:normal;font-weight:300 700;font-display:swap;src:url('${browser.runtime.getURL('/fonts/instrument-sans-latin-wght-normal.woff2')}') format('woff2-variations');}`;
	document.head.appendChild(style);
}

// externalSourceURL returns raw only when it is an http(s) URL pointing
// somewhere other than Pinterest itself; otherwise undefined.
function externalSourceURL(raw: string | null | undefined): string | undefined {
	if (!raw) return undefined;
	let u: URL;
	try {
		u = new URL(raw);
	} catch {
		return undefined;
	}
	if (u.protocol !== 'http:' && u.protocol !== 'https:') return undefined;
	if (isPinterestHost(u.hostname) || u.hostname.replace(/^www\./, '') === 'pin.it') {
		return undefined;
	}
	return raw;
}

// pinterestSourceUrl resolves a Pinterest pin's original external source from
// the closeup page. og:see_also carries it in the head; the visible
// clickthrough anchor is a second independent signal. Returns undefined when
// the pin has no external source (caller falls back to the pin URL).
function pinterestSourceUrl(): string | undefined {
	if (!/\/pin\/\d+/.test(location.pathname)) return undefined;

	const ogSeeAlso = document.querySelector<HTMLMetaElement>(
		'meta[property="og:see_also"]'
	)?.content;
	const fromMeta = externalSourceURL(ogSeeAlso);
	if (fromMeta) return fromMeta;

	const clickthrough = document.querySelector<HTMLAnchorElement>(
		'a[data-test-id="clickthrough-link"]'
	);
	return externalSourceURL(clickthrough?.href);
}

function extractSiteHints(img?: HTMLImageElement): SiteHints {
	const host = location.hostname.replace(/^www\./, '');
	if (host === 'cosmos.so') {
		const caption = document.querySelector<HTMLElement>('[data-testid="element-ml-caption"]');
		if (caption) return { attributionCredit: caption.textContent?.trim() };
	}
	if (isPinterestHost(location.hostname)) {
		const originUrl = pinterestSourceUrl();
		if (originUrl) return { originUrl };
	}
	return { imageOrigin: img ? detectImageOrigin(img) : undefined };
}

export default defineContentScript({
	matches: ['<all_urls>'],
	cssInjectionMode: 'ui',

	async main(ctx) {
		const ui = await createShadowRootUi(ctx, {
			name: 'currents-clipper',
			position: 'overlay',
			anchor: 'html',
			onMount(container) {
				return mount(App, { target: container });
			},
			onRemove(app) {
				if (app) unmount(app as ReturnType<typeof mount>);
			}
		});

		ui.mount();
		const host = ui.shadowHost as HTMLElement;
		host.style.setProperty('position', 'fixed', 'important');
		host.style.setProperty('top', '0', 'important');
		host.style.setProperty('left', '0', 'important');
		host.style.setProperty('width', '0', 'important');
		host.style.setProperty('height', '0', 'important');
		host.style.setProperty('z-index', '2147483647', 'important');
		host.style.setProperty('margin', '0', 'important');
		host.style.setProperty('padding', '0', 'important');
		host.style.setProperty('border', '0', 'important');
		let contextImage:
			{ url: string; alt: string; imageOrigin?: SiteHints['imageOrigin'] } | undefined;

		document.addEventListener(
			'contextmenu',
			(event) => {
				const target = event.target;
				const img =
					target instanceof HTMLImageElement
						? target
						: target instanceof Element
							? target.closest('img')
							: null;
				contextImage = img
					? { url: img.currentSrc || img.src, alt: img.alt, imageOrigin: detectImageOrigin(img) }
					: undefined;
			},
			true
		);

		browser.runtime.onMessage.addListener((message) => {
			if (message.type === 'SHOW_CLIPPER') {
				injectFont();
				const img = Array.from(document.images).find((img) =>
					[img.currentSrc, img.src, bestImageUrl(img)].includes(message.imgUrl)
				);
				const siteHints = extractSiteHints(img);
				if (contextImage && contextImage.url === message.imgUrl && contextImage.imageOrigin)
					siteHints.imageOrigin = contextImage.imageOrigin;
				const altText = contextImage && contextImage.url === message.imgUrl ? contextImage.alt : '';
				contextImage = undefined;
				showClipper({
					imgUrl: message.imgUrl ?? '',
					altText,
					originUrl: siteHints.originUrl ?? message.originUrl ?? '',
					pageTitle: message.pageTitle ?? '',
					siteHints
				});
				void loadClipperAuth();
			} else if (message.type === 'SHOW_CLIPPER_MULTI') {
				injectFont();
				contextImage = undefined;
				showClipper({
					mode: 'multi',
					candidates: collectPageImages(),
					originUrl: location.href,
					pageTitle: document.title
				});
				void loadClipperAuth();
			}
		});

		await loadSaveButtonSettings();
		const enabled = () => showButtonOnSite(location.hostname);
		const pinterest = isPinterestHost(location.hostname);
		const instagram = isInstagramHost(location.hostname);
		const threads = isThreadsHost(location.hostname);
		if (pinterest) injectFont();
		const clearButtons = pinterest
			? setupPinterestGrid(enabled)
			: setupGenericImageButton(
					enabled,
					(img) => {
						const candidate = candidateForImage(img);
						if (!candidate) return;
						injectFont();
						const siteHints = extractSiteHints(img);
						showClipper({
							imgUrl: candidate.url,
							altText: candidate.alt,
							originUrl:
								siteHints.originUrl ??
								(instagram ? img.closest<HTMLAnchorElement>('a[href*="/p/"]')?.href : undefined) ??
								(threads
									? img
											.closest<HTMLAnchorElement>('a[href*="/post/"]')
											?.href.replace(/\/media\/?$/, '')
									: undefined) ??
								location.href,
							pageTitle: document.title,
							siteHints
						});
						void loadClipperAuth();
					},
					instagram ? resolveInstagramImage : threads ? resolveThreadsImage : undefined
				);
		browser.storage.onChanged.addListener((changes, area) => {
			if (area === 'local' && changes.saveButtonSettings) clearButtons();
		});
	}
});

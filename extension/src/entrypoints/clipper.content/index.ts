import { mount, unmount } from 'svelte';
import App from './App.svelte';
import { clipper, showClipper, hideClipper, type SiteHints } from '../../lib/clipper-store.svelte';
import { setupPinterestGrid } from './pinterest-grid';
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

// Matches pinterest.com and every regional variant: country subdomains
// (it.pinterest.com), ccTLDs (pinterest.co.uk, pinterest.fr), and www.
function isPinterestHost(hostname: string): boolean {
  return /(^|\.)pinterest\.[a-z.]+$/.test(hostname);
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
    'meta[property="og:see_also"]',
  )?.content;
  const fromMeta = externalSourceURL(ogSeeAlso);
  if (fromMeta) return fromMeta;

  const clickthrough = document.querySelector<HTMLAnchorElement>(
    'a[data-test-id="clickthrough-link"]',
  );
  return externalSourceURL(clickthrough?.href);
}

function extractSiteHints(): SiteHints {
  const host = location.hostname.replace(/^www\./, '');
  if (host === 'cosmos.so') {
    const caption = document.querySelector<HTMLElement>('[data-testid="element-ml-caption"]');
    if (caption) return { attributionCredit: caption.textContent?.trim() };
  }
  if (isPinterestHost(location.hostname)) {
    const originUrl = pinterestSourceUrl();
    if (originUrl) return { originUrl };
  }
  return {};
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
      },
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

    browser.runtime.onMessage.addListener((message) => {
      if (message.type !== 'SHOW_CLIPPER') return;
      injectFont();
      const siteHints = extractSiteHints();
      // Open the dialog immediately, then resolve auth + collections in the
      // background (like the Pinterest save button) so it doesn't wait on a
      // network round-trip.
      showClipper({
        imgUrl: message.imgUrl ?? '',
        originUrl: siteHints.originUrl ?? message.originUrl ?? '',
        pageTitle: message.pageTitle ?? '',
        collections: [],
        collectionsLoading: true,
        authState: 'authenticated',
        userHandle: '',
        siteHints,
      });
      browser.runtime.sendMessage({ type: 'CHECK_AUTH' }).then((res) => {
        if (!clipper.visible) return; // dismissed while loading
        clipper.authState = res.authenticated ? 'authenticated' : 'unauthenticated';
        clipper.collections = res.authenticated ? res.collections : [];
        clipper.userHandle = res.authenticated ? res.handle : '';
        clipper.collectionsLoading = false;
      });
    });

    document.addEventListener('currents-clipper-close', () => hideClipper());

    if (isPinterestHost(location.hostname)) {
      injectFont();
      setupPinterestGrid();
    }
  },
});

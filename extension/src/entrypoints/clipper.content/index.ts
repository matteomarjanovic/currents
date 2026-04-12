import { mount, unmount } from 'svelte';
import App from './App.svelte';
import { showClipper, hideClipper, type SiteHints } from '../../lib/clipper-store.svelte';

function extractSiteHints(): SiteHints {
  const host = location.hostname.replace(/^www\./, '');
  if (host === 'cosmos.so') {
    const caption = document.querySelector<HTMLElement>('[data-testid="element-ml-caption"]');
    if (caption) return { attributionCredit: caption.textContent?.trim() };
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
      inheritStyles: true,
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
      showClipper({
        mode: message.mode ?? 'single',
        imgUrl: message.imgUrl ?? '',
        originUrl: message.originUrl ?? '',
        pageTitle: message.pageTitle ?? '',
        collections: message.collections,
        authState: message.authState,
        userHandle: message.userHandle,
        siteHints: extractSiteHints(),
        pinCount: message.pinCount ?? 0,
        defaultCollectionDescription: message.defaultCollectionDescription ?? '',
      });
    });

    // App.svelte dispatches this event on document when the user closes the modal
    document.addEventListener('currents-clipper-close', () => hideClipper());
  },
});

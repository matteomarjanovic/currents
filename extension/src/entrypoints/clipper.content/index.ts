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
      position: 'inline',
      anchor: 'body',
      onMount(container) {
        return mount(App, { target: container });
      },
      onRemove(app) {
        if (app) unmount(app as ReturnType<typeof mount>);
      },
    });

    ui.mount();

    browser.runtime.onMessage.addListener((message) => {
      if (message.type !== 'SHOW_CLIPPER') return;
      showClipper({
        imgUrl: message.imgUrl,
        originUrl: message.originUrl,
        pageTitle: message.pageTitle,
        collections: message.collections,
        authState: message.authState,
        userHandle: message.userHandle,
        siteHints: extractSiteHints(),
      });
    });

    // App.svelte dispatches this event on document when the user closes the modal
    document.addEventListener('currents-clipper-close', () => hideClipper());
  },
});

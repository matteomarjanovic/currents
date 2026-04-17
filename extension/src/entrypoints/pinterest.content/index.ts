import logoSvg from '../../assets/logo.svg?raw';

const CONCURRENCY = 3;

export default defineContentScript({
  matches: ['*://*.pinterest.com/*'],
  runAt: 'document_idle',

  main(_ctx) {
    const queuedPinIds = new Set<string>();
    let collectionUri: string | null = null;
    let collectionName = '';
    let collectionAttributionCredit = '';
    let cancelled = false;
    let successCount = 0;
    let errorCount = 0;
    let buttonEl: HTMLElement | null = null;
    let bannerEl: HTMLElement | null = null;
    let pinObserver: MutationObserver | null = null;
    const queue: Array<() => Promise<void>> = [];
    let active = 0;

    // --- helpers ---

    function isBoardPage(): boolean {
      const parts = location.pathname.split('/').filter(Boolean);
      if (parts.length !== 2 || parts[0] === 'pin') return false;
      return !!document.querySelector('h1#board-name');
    }

    function toOriginal(url: string): string {
      return url.replace(/(i\.pinimg\.com\/)\d+x\d*\//, '$1originals/');
    }

    function pickImgUrl(pin: HTMLElement): string | null {
      const img = pin.querySelector<HTMLImageElement>('img[srcset], img[src]');
      if (!img) return null;
      const srcset = img.getAttribute('srcset');
      let url = '';
      if (srcset) {
        const entries = srcset.split(',').map((s) => s.trim());
        url = entries[entries.length - 1]?.split(/\s+/)[0] ?? '';
      } else {
        url = img.src;
      }
      if (!url || !/i\.pinimg\.com/.test(url)) return null;
      return toOriginal(url);
    }

    function pickOriginUrl(pin: HTMLElement): string {
      const a = pin.querySelector<HTMLAnchorElement>('a[href^="/pin/"]');
      const href = a?.getAttribute('href');
      return href ? new URL(href, location.origin).href : location.href;
    }

    function escapeHtml(s: string) {
      return s.replace(
        /[&<>"']/g,
        (c) =>
          ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]!),
      );
    }

    // --- per-pin overlay ---

    function ensureOverlay(pin: HTMLElement): HTMLElement {
      let overlay = pin.querySelector<HTMLElement>(':scope > .currents-pin-overlay');
      if (overlay) return overlay;
      if (getComputedStyle(pin).position === 'static') pin.style.position = 'relative';
      overlay = document.createElement('div');
      overlay.className = 'currents-pin-overlay';
      overlay.style.cssText =
        'position:absolute;bottom:8px;right:8px;width:28px;height:28px;border-radius:50%;display:flex;align-items:center;justify-content:center;background:rgba(0,0,0,0.6);color:#fff;font-size:14px;z-index:10;pointer-events:none;font-family:system-ui,-apple-system,sans-serif;';
      pin.appendChild(overlay);
      return overlay;
    }

    function setOverlay(pin: HTMLElement, kind: 'queued' | 'saving' | 'saved' | 'error') {
      const overlay = ensureOverlay(pin);
      if (kind === 'queued') {
        overlay.style.background = 'rgba(0,0,0,0.5)';
        overlay.innerHTML = '<span style="width:8px;height:8px;background:#fff;border-radius:50%;"></span>';
      } else if (kind === 'saving') {
        overlay.style.background = 'rgba(0,0,0,0.65)';
        overlay.innerHTML =
          '<span style="width:14px;height:14px;border:2px solid #fff;border-top-color:transparent;border-radius:50%;display:inline-block;animation:currents-spin 0.8s linear infinite;"></span>';
      } else if (kind === 'saved') {
        overlay.style.background = 'rgba(34,150,57,0.95)';
        overlay.innerHTML = '<span style="font-weight:700;">✓</span>';
      } else {
        overlay.style.background = 'rgba(192,0,0,0.95)';
        overlay.innerHTML = '<span style="font-weight:700;">!</span>';
      }
    }

    function injectKeyframes() {
      if (document.getElementById('currents-style')) return;
      const s = document.createElement('style');
      s.id = 'currents-style';
      s.textContent = '@keyframes currents-spin { to { transform: rotate(360deg); } }';
      document.head.appendChild(s);
    }

    // --- button ---

    function injectButton() {
      if (buttonEl && document.contains(buttonEl)) return;
      const heading = document.querySelector<HTMLElement>('h1#board-name');
      const anchor =
        document.querySelector<HTMLElement>('div[data-test-id="board-header"]') ??
        heading?.parentElement ??
        null;
      if (!anchor) return;
      const btn = document.createElement('button');
      btn.id = 'currents-board-btn';
      btn.type = 'button';
      btn.style.cssText =
        'display:inline-flex;align-items:center;gap:8px;padding:10px 16px;margin-top:12px;background:#000;color:#fff;border:none;border-radius:999px;cursor:pointer;font-family:inherit;font-size:14px;font-weight:600;';
      const logoSpan = document.createElement('span');
      logoSpan.style.cssText = 'height:14px;display:inline-flex;color:#fff;';
      logoSpan.innerHTML = logoSvg as string;
      const svg = logoSpan.querySelector('svg');
      if (svg) {
        svg.setAttribute('height', '14');
        svg.removeAttribute('width');
        svg.style.height = '14px';
        svg.style.width = 'auto';
      }
      btn.appendChild(document.createTextNode('Save board to '));
      btn.appendChild(logoSpan);
      btn.addEventListener('click', onButtonClick);
      anchor.appendChild(btn);
      buttonEl = btn;
    }

    function removeButton() {
      buttonEl?.remove();
      buttonEl = null;
    }

    // --- banner ---

    function updateBanner() {
      if (!bannerEl || !document.contains(bannerEl)) {
        bannerEl = document.createElement('div');
        bannerEl.id = 'currents-board-banner';
        bannerEl.style.cssText =
          'position:fixed;bottom:20px;left:20px;z-index:2147483646;background:#111;color:#fff;padding:14px 18px;border-radius:12px;box-shadow:0 8px 32px rgba(0,0,0,0.3);font-family:system-ui,-apple-system,sans-serif;font-size:13px;display:flex;flex-direction:column;gap:6px;max-width:320px;line-height:1.4;';
        document.body.appendChild(bannerEl);
      }
      const total = queuedPinIds.size;
      const pending = total - successCount - errorCount;
      bannerEl.innerHTML = `
        <div style="font-weight:600;">Importing into "${escapeHtml(collectionName || 'collection')}"</div>
        <div>${successCount} saved · ${pending} pending${errorCount ? ` · ${errorCount} failed` : ''}</div>
        <div style="opacity:0.75;">Scroll the board to load more pins — we'll save them automatically.</div>
      `;
      const stop = document.createElement('button');
      stop.textContent = 'Stop importing';
      stop.style.cssText =
        'margin-top:6px;padding:6px 10px;background:transparent;color:#fff;border:1px solid rgba(255,255,255,0.4);border-radius:6px;cursor:pointer;font-family:inherit;font-size:12px;align-self:flex-start;';
      stop.addEventListener('click', stopImport);
      bannerEl.appendChild(stop);
    }

    function stopImport() {
      cancelled = true;
      collectionUri = null;
      collectionAttributionCredit = '';
      pinObserver?.disconnect();
      pinObserver = null;
      bannerEl?.remove();
      bannerEl = null;
      queue.length = 0;
    }

    // --- queue ---

    function enqueuePin(pin: HTMLElement) {
      if (cancelled || !collectionUri) return;
      const pinId = pin.getAttribute('data-test-pin-id');
      if (!pinId || queuedPinIds.has(pinId)) return;
      const imgUrl = pickImgUrl(pin);
      if (!imgUrl) return;
      queuedPinIds.add(pinId);
      setOverlay(pin, 'queued');
      const originUrl = pickOriginUrl(pin);
      const uri = collectionUri;
      queue.push(async () => {
        if (cancelled) return;
        setOverlay(pin, 'saving');
        try {
          const resp = await browser.runtime.sendMessage({
            type: 'SAVE_IMAGE',
            imgUrl,
            collectionUri: uri,
            text: '',
            originUrl,
            attributionUrl: '',
            attributionLicense: '',
            attributionCredit: collectionAttributionCredit,
          });
          if (resp?.ok) {
            setOverlay(pin, 'saved');
            successCount++;
          } else {
            setOverlay(pin, 'error');
            errorCount++;
          }
        } catch {
          setOverlay(pin, 'error');
          errorCount++;
        }
        updateBanner();
      });
      updateBanner();
      pump();
    }

    function pump() {
      while (active < CONCURRENCY && queue.length > 0 && !cancelled) {
        const job = queue.shift()!;
        active++;
        job().finally(() => {
          active--;
          pump();
        });
      }
    }

    function scanPins() {
      document
        .querySelectorAll<HTMLElement>('div[data-test-id="pin"]')
        .forEach(enqueuePin);
    }

    function startPinObserver() {
      if (pinObserver) return;
      pinObserver = new MutationObserver(() => scanPins());
      pinObserver.observe(document.body, { childList: true, subtree: true });
    }

    // --- handlers ---

    function onButtonClick() {
      const boardName = document.querySelector('h1#board-name')?.textContent?.trim() ?? '';
      const suggested = boardName
        ? `Imported from Pinterest board "${boardName}" (${location.href})`
        : `Imported from Pinterest board ${location.href}`;
      browser.runtime.sendMessage({
        type: 'OPEN_BOARD_PICKER',
        boardName,
        originUrl: location.href,
        pinCount: document.querySelectorAll('div[data-test-id="pin"]').length,
        defaultCollectionDescription: suggested,
      });
    }

    document.addEventListener('currents-board-confirm', ((e: Event) => {
      const detail = (e as CustomEvent).detail as {
        collectionUri: string;
        collectionName: string;
        attributionCredit: string;
      };
      collectionUri = detail.collectionUri;
      collectionName = detail.collectionName;
      collectionAttributionCredit = detail.attributionCredit ?? '';
      cancelled = false;
      successCount = 0;
      errorCount = 0;
      queuedPinIds.clear();
      queue.length = 0;
      injectKeyframes();
      updateBanner();
      scanPins();
      startPinObserver();
    }) as EventListener);

    // --- SPA navigation watcher ---

    let lastPath = location.pathname;
    function checkPage() {
      if (location.pathname !== lastPath) {
        lastPath = location.pathname;
        removeButton();
      }
      if (isBoardPage()) injectButton();
      else removeButton();
    }

    const pageObserver = new MutationObserver(() => checkPage());
    pageObserver.observe(document.body, { childList: true, subtree: true });
    checkPage();
  },
});

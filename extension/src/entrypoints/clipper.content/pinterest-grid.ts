import { clipper, showClipper } from '../../lib/clipper-store.svelte';

// Hover button injected into Pinterest pins that opens the Currents save dialog
// for the pin's image — both masonry grid items and the closeup (single pin)
// hero image. Buttons are attached lazily via mouseover delegation, so the
// virtualized grid needs no MutationObserver.

const BTN_CLASS = 'currents-grid-save';

// Grid pins are wrapped in `[data-test-id="pin"]` inside a `[data-grid-item]`.
const GRID_PIN = '[data-test-id="pin"]';
// The closeup (single pin) hero markup differs by auth state and locale:
// logged-out serves `pin-closeup-image`, logged-in serves `closeup-image`
// (which carries a radial mask + border-radius that would fade/clip a corner
// button) wrapped in `closeup-body-image-container`. We detect any of these,
// then attach to a non-masked wrapper — preferring `closeup-body-image-container`.
const CLOSEUP_ANY =
  '[data-test-id="closeup-body-image-container"],[data-test-id="closeup-image"],[data-test-id="pin-closeup-image"]';
const CLOSEUP_HOSTS = [
  '[data-test-id="closeup-body-image-container"]',
  '[data-test-id="pin-closeup-image"]',
];

const HOVER_REVEAL = [GRID_PIN, ...CLOSEUP_ANY.split(',')]
  .map((s) => `${s}:hover .${BTN_CLASS}`)
  .join(',');

const STYLE = `
.${BTN_CLASS}{position:absolute;left:12px;bottom:12px;z-index:100;width:36px;height:36px;box-sizing:border-box;border-radius:50%;border:none;margin:0;padding:0;display:flex;align-items:center;justify-content:center;cursor:pointer;background:#fff;box-shadow:0 1px 4px rgba(0,0,0,.3);opacity:0;pointer-events:none;transition:opacity .1s,transform .1s}
${HOVER_REVEAL}{opacity:1;pointer-events:auto}
.${BTN_CLASS}:hover{transform:scale(1.08)}
.${BTN_CLASS} img{width:26px;height:26px;display:block}
/* Make room: shift Pinterest's "Visit site" chip right of our button so the
   row reads [Currents] [Visit site] ... [Share]. */
${GRID_PIN}:has(.${BTN_CLASS}) [data-test-id="pinrep-source-link"]{margin-left:44px;white-space:nowrap}
`;

// Largest entry in the srcset (Pinterest lists 236x → originals), falling
// back to the rendered src.
function bestImageUrl(img: HTMLImageElement): string {
  let best = img.currentSrc || img.src;
  let bestDesc = 0;
  for (const part of (img.srcset ?? '').split(',')) {
    const [url, desc] = part.trim().split(/\s+/);
    const d = parseFloat(desc) || 0;
    if (url && d > bestDesc) {
      bestDesc = d;
      best = url;
    }
  }
  return best;
}

async function openClipper(host: HTMLElement) {
  const img = host.querySelector('img');
  if (!img) return;
  // Grid pins carry the id directly; on the closeup it's in the URL.
  const pinId =
    host.getAttribute('data-test-pin-id') ?? location.pathname.match(/\/pin\/(\d+)/)?.[1] ?? null;
  // Open the dialog immediately; auth + collections resolve in the background
  // (optimistically assume signed in — the common case for the save button).
  showClipper({
    imgUrl: bestImageUrl(img),
    originUrl: pinId ? `https://www.pinterest.com/pin/${pinId}/` : location.href,
    pageTitle: document.title,
    collections: [],
    collectionsLoading: true,
    authState: 'authenticated',
    userHandle: '',
    siteHints: {},
  });
  const res = await browser.runtime.sendMessage({ type: 'CHECK_AUTH' });
  if (!clipper.visible) return; // dismissed while loading
  clipper.authState = res.authenticated ? 'authenticated' : 'unauthenticated';
  clipper.collections = res.authenticated ? res.collections : [];
  clipper.userHandle = res.authenticated ? res.handle : '';
  clipper.collectionsLoading = false;
}

// Resolve the hover target to the element we attach the button to, or null when
// it isn't a saveable image surface. Grid pins must sit in the masonry grid;
// for the closeup we walk up to the best (non-masked) wrapper.
function resolveHost(target: Element | null): HTMLElement | null {
  const pin = target?.closest?.<HTMLElement>(GRID_PIN);
  if (pin && pin.closest('[data-grid-item="true"]')) return pin;

  const cu = target?.closest?.<HTMLElement>(CLOSEUP_ANY);
  if (!cu) return null;
  for (const sel of CLOSEUP_HOSTS) {
    const h = cu.closest<HTMLElement>(sel);
    if (h) return h;
  }
  return cu;
}

export function setupPinterestGrid() {
  const style = document.createElement('style');
  style.textContent = STYLE;
  document.head.appendChild(style);

  document.addEventListener(
    'mouseover',
    (e) => {
      const host = resolveHost(e.target as Element | null);
      // Attach once per surface.
      if (!host || host.querySelector(`.${BTN_CLASS}`)) return;

      const btn = document.createElement('button');
      btn.className = BTN_CLASS;
      btn.type = 'button';
      btn.title = 'Save to Currents';
      const icon = document.createElement('img');
      icon.src = browser.runtime.getURL('/icon/currents.svg');
      icon.alt = '';
      btn.appendChild(icon);

      // Keep the click from reaching Pinterest's pin-open handlers.
      const stop = (ev: Event) => {
        ev.preventDefault();
        ev.stopPropagation();
      };
      btn.addEventListener('pointerdown', stop);
      btn.addEventListener('mousedown', stop);
      btn.addEventListener('click', (ev) => {
        stop(ev);
        openClipper(host);
      });

      // Grid pins anchor on the image wrapper; the closeup attaches to its
      // wrapper directly. Ensure the anchor is positioned so the absolute button
      // lands at the image's bottom-left.
      const isCloseup = !host.matches(GRID_PIN);
      const anchor = host.querySelector<HTMLElement>('[data-test-id="pinWrapper"]') ?? host;
      if (getComputedStyle(anchor).position === 'static') anchor.style.position = 'relative';
      anchor.appendChild(btn);

      // Grid wrappers hug the image, so the CSS inset is already correct. On the
      // closeup the image is centered inside a wider container, so align the
      // button to the image's own edges and keep it aligned as layout changes.
      const img = anchor.querySelector('img');
      if (isCloseup && img) {
        const place = () => {
          const a = anchor.getBoundingClientRect();
          const i = img.getBoundingClientRect();
          btn.style.left = `${i.left - a.left + 12}px`;
          btn.style.bottom = `${a.bottom - i.bottom + 12}px`;
        };
        place();
        const ro = new ResizeObserver(() => {
          if (!btn.isConnected) return ro.disconnect();
          place();
        });
        ro.observe(anchor);
        ro.observe(img);
      }
    },
    { passive: true }
  );
}

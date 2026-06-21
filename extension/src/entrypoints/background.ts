import { blobCidFromBytes } from '../lib/blob-cid';

const CURRENTS_URL = import.meta.env.VITE_CURRENTS_URL ?? 'https://api.currents.is';
const FRONTEND_URL = import.meta.env.VITE_CURRENTS_FRONTEND_URL ?? 'https://currents.is';
const AUTH_TTL_MS = 60_000;

interface AuthCache {
  did: string;
  handle: string;
  fetchedAt: number;
}

interface Collection {
  uri: string;
  name: string;
  saveCount: number;
  parentUri?: string;
  previews?: { url: string; labels?: string[] }[];
  createdAt?: string;
  lastSavedAt?: string;
}

// --- Cookie helper ---
// Service workers don't share the browser's cookie jar, so we read the
// session cookie via the cookies API and attach it manually.

async function getSessionCookie(): Promise<string | null> {
  const url = new URL(CURRENTS_URL);
  const cookie = await browser.cookies.get({
    url: CURRENTS_URL,
    name: 'currents-session',
  });
  return cookie?.value ? `currents-session=${cookie.value}` : null;
}

async function appviewFetch(url: string, init: RequestInit = {}): Promise<Response> {
  const cookie = await getSessionCookie();
  const headers = new Headers(init.headers);
  if (cookie) headers.set('Cookie', cookie);
  return fetch(url, { ...init, headers });
}

// --- Auth helpers ---

async function getCachedAuth(): Promise<AuthCache | null> {
  const result = await browser.storage.session.get('authCache');
  const cache = result.authCache as AuthCache | undefined;
  if (!cache || Date.now() - cache.fetchedAt > AUTH_TTL_MS) return null;
  return cache;
}

async function fetchAuth(): Promise<AuthCache | null> {
  try {
    const url = `${CURRENTS_URL}/api/me`;
    const cookie = await getSessionCookie();
    console.log('[currents] fetchAuth →', url, 'cookie:', cookie ? cookie.substring(0, 40) + '...' : 'null');
    const resp = await appviewFetch(url);
    const text = await resp.text();
    console.log('[currents] fetchAuth status', resp.status, 'body:', text.substring(0, 200));
    if (!resp.ok) return null;
    const data = JSON.parse(text);
    console.log('[currents] fetchAuth data', data);
    const cache: AuthCache = { did: data.did, handle: data.handle, fetchedAt: Date.now() };
    await browser.storage.session.set({ authCache: cache });
    return cache;
  } catch (e) {
    console.error('[currents] fetchAuth error', e);
    return null;
  }
}

async function getAuth(): Promise<AuthCache | null> {
  return (await getCachedAuth()) ?? (await fetchAuth());
}

async function fetchCollections(did: string): Promise<Collection[]> {
  try {
    const url = `${CURRENTS_URL}/xrpc/is.currents.feed.getActorCollections?actor=${encodeURIComponent(did)}&limit=100`;
    const resp = await appviewFetch(url);
    if (!resp.ok) return [];
    const data = await resp.json();
    return (data.collections ?? []).map((c: any) => ({
      uri: c.uri,
      name: c.name,
      saveCount: c.saveCount ?? 0,
      parentUri: c.parentUri,
      previews: c.previews,
      createdAt: c.createdAt,
      lastSavedAt: c.lastSavedAt,
    }));
  } catch {
    return [];
  }
}

// --- Save handler ---

async function handleCreateCollection(message: {
  name: string;
  description?: string;
  parent?: string;
}): Promise<{ ok: boolean; uri?: string; error?: string }> {
  const auth = await getAuth();
  if (!auth) return { ok: false, error: 'Not logged in', authError: true };

  const body = new URLSearchParams({ name: message.name });
  if (message.description) body.set('description', message.description);
  if (message.parent) body.set('parent', message.parent);

  try {
    const resp = await appviewFetch(`${CURRENTS_URL}/collection`, {
      method: 'POST',
      body,
      headers: { Accept: 'application/json' },
      redirect: 'manual',
    });
    // JSON response from updated server
    const ct = resp.headers.get('Content-Type') ?? '';
    if (ct.includes('application/json')) {
      const data = await resp.json();
      return { ok: true, uri: data.uri };
    }
    // Fallback: server returned a redirect (older server without JSON support)
    if (resp.type === 'opaqueredirect' || resp.status === 0 || (resp.status >= 300 && resp.status < 400)) {
      const collections = await fetchCollections(auth.did);
      const created = collections.find((c) => c.name === message.name);
      return { ok: true, uri: created?.uri };
    }
    const errorText = await resp.text();
    return { ok: false, error: errorText.trim() || `HTTP ${resp.status}` };
  } catch (e) {
    return { ok: false, error: String(e) };
  }
}

async function handleSave(message: {
  imgUrl: string;
  collectionUri: string;
  text: string;
  alt?: string;
  originUrl: string;
  attributionUrl?: string;
  attributionLicense?: string;
  attributionCredit?: string;
  labels?: string;
}): Promise<{ ok: boolean; error?: string }> {
  const auth = await getAuth();
  if (!auth) return { ok: false, error: 'Not logged in', authError: true };

  let imageBlob: Blob;
  try {
    imageBlob = await fetchImageBlob(message.imgUrl);
  } catch (e) {
    return { ok: false, error: `Could not fetch image: ${e}` };
  }

  const filename = message.imgUrl.startsWith('data:')
    ? 'image.jpg'
    : (new URL(message.imgUrl).pathname.split('/').pop() ?? 'image.jpg');

  const form = new FormData();
  form.append('image', imageBlob, filename);
  form.append('collection', message.collectionUri);
  if (message.text) form.append('title', message.text);
  if (message.alt) form.append('alt', message.alt);
  if (message.originUrl) form.append('url', message.originUrl);
  if (message.attributionUrl) form.append('attribution_url', message.attributionUrl);
  if (message.attributionLicense) form.append('attribution_license', message.attributionLicense);
  if (message.attributionCredit) form.append('attribution_credit', message.attributionCredit);
  if (message.labels) form.append('labels', message.labels);

  try {
    const resp = await appviewFetch(`${CURRENTS_URL}/save`, {
      method: 'POST',
      body: form,
      redirect: 'manual',
    });
    // POST /save returns 302 on success; redirect: 'manual' gives us an opaque redirect
    if (resp.type === 'opaqueredirect' || resp.status === 0) {
      return { ok: true };
    }
    // PDS rate-limit: the user's data server is throttling blob uploads.
    if (resp.status === 429) {
      return {
        ok: false,
        error: 'Your data server is temporarily limiting uploads. Please try again in a few minutes.',
      };
    }
    const errorText = await resp.text();
    return { ok: false, error: errorText.trim() || `HTTP ${resp.status}` };
  } catch (e) {
    return { ok: false, error: String(e) };
  }
}

async function fetchImageBlob(imgUrl: string): Promise<Blob> {
  if (imgUrl.startsWith('data:')) {
    // data URI — decode directly without a network fetch
    const [header, b64] = imgUrl.split(',');
    const mime = header.match(/:(.*?);/)?.[1] ?? 'image/jpeg';
    const bytes = Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
    return new Blob([bytes], { type: mime });
  }
  const imgResp = await fetch(imgUrl);
  if (!imgResp.ok) throw new Error(`HTTP ${imgResp.status}`);
  return imgResp.blob();
}

// Looks up an existing alt text for the image at imgUrl (matched by exact blob
// CID) so the clipper can pre-fill the field. Best-effort: returns '' on any error.
async function handleLookupAlt(message: { imgUrl: string }): Promise<{ alt: string }> {
  try {
    const auth = await getAuth();
    if (!auth) return { alt: '' };
    const blob = await fetchImageBlob(message.imgUrl);
    const cid = await blobCidFromBytes(await blob.arrayBuffer());
    const resp = await appviewFetch(`${CURRENTS_URL}/api/blob/alt?cid=${encodeURIComponent(cid)}`);
    if (!resp.ok) return { alt: '' };
    const data = await resp.json();
    return { alt: typeof data.alt === 'string' ? data.alt : '' };
  } catch {
    return { alt: '' };
  }
}

// --- Entry point ---

export default defineBackground(() => {
  // Create the context menu once on install to avoid duplicates across SW restarts
  browser.runtime.onInstalled.addListener(() => {
    browser.contextMenus.create({
      id: 'save-to-currents',
      title: 'Save to Currents',
      contexts: ['image'],
    });
  });

  browser.contextMenus.onClicked.addListener(async (info, tab) => {
    if (info.menuItemId !== 'save-to-currents' || !info.srcUrl || !tab?.id) return;
    // Open the dialog immediately; the content script resolves auth + collections
    // in the background (via CHECK_AUTH), matching the Pinterest save button.
    try {
      await browser.tabs.sendMessage(tab.id, {
        type: 'SHOW_CLIPPER',
        imgUrl: info.srcUrl,
        originUrl: tab.url ?? '',
        pageTitle: tab.title ?? '',
      });
    } catch (e) {
      console.error('[currents] sendMessage failed', e);
    }
  });

  browser.cookies.onChanged.addListener(async (changeInfo) => {
    if (changeInfo.cookie.name !== 'currents-session') return;
    if (changeInfo.removed) {
      await browser.storage.session.remove('authCache');
    }
  });

  // Auto-close the login tab once OAuth lands on the success page. The page
  // shows a 5s countdown, so we close it after the same delay.
  browser.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
    if (changeInfo.status === 'complete' && tab.url?.startsWith(`${FRONTEND_URL}/login/success`)) {
      setTimeout(() => browser.tabs.remove(tabId), 5000);
    }
  });

  browser.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (message.type === 'SAVE_IMAGE') {
      handleSave(message).then(sendResponse);
      return true;
    }
    if (message.type === 'LOOKUP_ALT') {
      handleLookupAlt(message).then(sendResponse);
      return true;
    }
    if (message.type === 'CREATE_COLLECTION') {
      handleCreateCollection(message).then(sendResponse);
      return true;
    }
    if (message.type === 'CHECK_AUTH') {
      (async () => {
        await browser.storage.session.remove('authCache');
        const auth = await fetchAuth();
        if (!auth) { sendResponse({ authenticated: false }); return; }
        const collections = await fetchCollections(auth.did);
        sendResponse({ authenticated: true, handle: auth.handle, collections });
      })();
      return true;
    }
  });
});

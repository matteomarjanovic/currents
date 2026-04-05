const CURRENTS_URL = import.meta.env.VITE_CURRENTS_URL ?? 'https://currents.is';
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
    }));
  } catch {
    return [];
  }
}

// --- Save handler ---

async function handleCreateCollection(message: {
  name: string;
}): Promise<{ ok: boolean; uri?: string; error?: string }> {
  const auth = await getAuth();
  if (!auth) return { ok: false, error: 'Not logged in', authError: true };

  const body = new URLSearchParams({ name: message.name });

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
  originUrl: string;
  attributionUrl?: string;
  attributionLicense?: string;
  attributionCredit?: string;
}): Promise<{ ok: boolean; error?: string }> {
  const auth = await getAuth();
  if (!auth) return { ok: false, error: 'Not logged in', authError: true };

  let imageBlob: Blob;
  try {
    if (message.imgUrl.startsWith('data:')) {
      // data URI — decode directly without a network fetch
      const [header, b64] = message.imgUrl.split(',');
      const mime = header.match(/:(.*?);/)?.[1] ?? 'image/jpeg';
      const bytes = Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
      imageBlob = new Blob([bytes], { type: mime });
    } else {
      const imgResp = await fetch(message.imgUrl);
      if (!imgResp.ok) throw new Error(`HTTP ${imgResp.status}`);
      imageBlob = await imgResp.blob();
    }
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
  if (message.originUrl) form.append('url', message.originUrl);
  if (message.attributionUrl) form.append('attribution_url', message.attributionUrl);
  if (message.attributionLicense) form.append('attribution_license', message.attributionLicense);
  if (message.attributionCredit) form.append('attribution_credit', message.attributionCredit);

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
    const errorText = await resp.text();
    return { ok: false, error: errorText.trim() || `HTTP ${resp.status}` };
  } catch (e) {
    return { ok: false, error: String(e) };
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
    console.log('[currents] contextMenu clicked', info.menuItemId, info.srcUrl, tab?.id);
    if (info.menuItemId !== 'save-to-currents' || !info.srcUrl || !tab?.id) return;

    const auth = await getAuth();
    console.log('[currents] auth result', auth ? `did=${auth.did}` : 'null');
    const collections: Collection[] = auth ? await fetchCollections(auth.did) : [];
    console.log('[currents] collections', collections.length);

    try {
      await browser.tabs.sendMessage(tab.id, {
        type: 'SHOW_CLIPPER',
        imgUrl: info.srcUrl,
        originUrl: tab.url ?? '',
        pageTitle: tab.title ?? '',
        collections,
        authState: auth ? 'authenticated' : 'unauthenticated',
        userHandle: auth?.handle ?? '',
      });
      console.log('[currents] sendMessage succeeded');
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

  browser.runtime.onMessage.addListener((message, _sender, sendResponse) => {
    if (message.type === 'SAVE_IMAGE') {
      handleSave(message).then(sendResponse);
      return true;
    }
    if (message.type === 'CREATE_COLLECTION') {
      handleCreateCollection(message).then(sendResponse);
      return true;
    }
  });
});

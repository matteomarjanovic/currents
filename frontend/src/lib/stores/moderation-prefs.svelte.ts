import { PUBLIC_APPVIEW_URL } from '$env/static/public';
import type { LabelView } from '$lib/types';

export type AdultVisibility = 'show' | 'blur' | 'hide';
export type AiVisibility = 'show' | 'hide';

export type AdultKey = 'porn' | 'sexual' | 'nudity' | 'graphicMedia';

interface Prefs {
	porn: AdultVisibility;
	sexual: AdultVisibility;
	nudity: AdultVisibility;
	graphicMedia: AdultVisibility;
	aiGenerated: AiVisibility;
}

// Defaults mirror the appview DB column defaults (migration 025). Used until the
// server load completes, and as the resting state for users who never change them.
const DEFAULTS: Prefs = {
	porn: 'blur',
	sexual: 'blur',
	nudity: 'blur',
	graphicMedia: 'blur',
	aiGenerated: 'show'
};

function isAdult(v: unknown): v is AdultVisibility {
	return v === 'show' || v === 'blur' || v === 'hide';
}
function isAi(v: unknown): v is AiVisibility {
	return v === 'show' || v === 'hide';
}

/**
 * Reactive viewer preferences for moderation rendering. Server-backed so they
 * follow the user across browsers and devices. Consumed by <LabeledMedia>
 * (blur + badge) and the upstream filter via shouldHide().
 */
export const modPrefs = $state<Prefs>({ ...DEFAULTS });
export const modPrefsLoaded = $state({ value: false });

export async function loadModerationPrefs() {
	try {
		const res = await fetch(`${PUBLIC_APPVIEW_URL}/api/moderation/prefs`, {
			credentials: 'include'
		});
		if (!res.ok) return;
		const data = (await res.json()) as Partial<Prefs>;
		if (isAdult(data.porn)) modPrefs.porn = data.porn;
		if (isAdult(data.sexual)) modPrefs.sexual = data.sexual;
		if (isAdult(data.nudity)) modPrefs.nudity = data.nudity;
		if (isAdult(data.graphicMedia)) modPrefs.graphicMedia = data.graphicMedia;
		if (isAi(data.aiGenerated)) modPrefs.aiGenerated = data.aiGenerated;
		modPrefsLoaded.value = true;
	} catch {
		// best-effort; the defaults remain in effect until a later load
	}
}

async function persist() {
	try {
		await fetch(`${PUBLIC_APPVIEW_URL}/api/moderation/prefs`, {
			method: 'PUT',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				porn: modPrefs.porn,
				sexual: modPrefs.sexual,
				nudity: modPrefs.nudity,
				graphicMedia: modPrefs.graphicMedia,
				aiGenerated: modPrefs.aiGenerated
			})
		});
	} catch {
		// best-effort; will resync from the server on next load
	}
}

export function setAdult(key: AdultKey, val: AdultVisibility) {
	modPrefs[key] = val; // optimistic
	void persist();
}

export function setAi(val: AiVisibility) {
	modPrefs.aiGenerated = val; // optimistic
	void persist();
}

// ─────────────────────────────────────────────────────────────────────────────
// Resolution helpers used by LabeledMedia (blur/badge) and MasonryGrid + save
// detail (upstream filter).

const RESTRICTIVENESS: Record<'show' | 'blur' | 'hide', number> = {
	show: 0,
	blur: 1,
	hide: 2
};

function mostRestrictive(
	a: 'show' | 'blur' | 'hide',
	b: 'show' | 'blur' | 'hide'
): 'show' | 'blur' | 'hide' {
	return RESTRICTIVENESS[a] >= RESTRICTIVENESS[b] ? a : b;
}

/**
 * Map an atproto label value to the viewer's chosen visibility under the
 * current prefs. Unknown / non-moderation labels return 'show' (no-op).
 *
 * Special cases:
 *  - `!hide` (takedown): always 'hide' regardless of prefs (defense in depth;
 *    backend usually filters these at the SQL layer first).
 *  - `currents-ai-generated`: AiVisibility is 2-state; "show" is treated as
 *    show, "hide" as hide. Blur is not an option for AI.
 */
export function visibilityFor(labelVal: string): 'show' | 'blur' | 'hide' {
	switch (labelVal) {
		case '!hide':
			return 'hide';
		case 'porn':
			return modPrefs.porn;
		case 'sexual':
			return modPrefs.sexual;
		case 'nudity':
			return modPrefs.nudity;
		case 'graphic-media':
			return modPrefs.graphicMedia;
		case 'currents-ai-generated':
			return modPrefs.aiGenerated;
		default:
			return 'show';
	}
}

/**
 * Whether a save should be filtered out of the user's view entirely (no card,
 * no image fetch). Called by MasonryGrid and save-detail.
 */
export function shouldHide(labels?: LabelView[]): boolean {
	if (!labels || labels.length === 0) return false;
	for (const l of labels) {
		if (visibilityFor(l.val) === 'hide') return true;
	}
	return false;
}

/**
 * The effective visibility for a save's labels — the most restrictive
 * across all of them. Used by LabeledMedia to decide between blur and show.
 */
export function effectiveVisibility(labels?: LabelView[]): 'show' | 'blur' | 'hide' {
	if (!labels || labels.length === 0) return 'show';
	let acc: 'show' | 'blur' | 'hide' = 'show';
	for (const l of labels) {
		acc = mostRestrictive(acc, visibilityFor(l.val));
		if (acc === 'hide') break;
	}
	return acc;
}

import { browser } from '$app/environment';
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

const STORAGE_KEY = 'currents-mod-prefs-v2';
const DEFAULTS: Prefs = {
	porn: 'blur',
	sexual: 'blur',
	nudity: 'blur',
	graphicMedia: 'blur',
	aiGenerated: 'hide'
};

function isAdult(v: unknown): v is AdultVisibility {
	return v === 'show' || v === 'blur' || v === 'hide';
}
function isAi(v: unknown): v is AiVisibility {
	return v === 'show' || v === 'hide';
}

function load(): Prefs {
	if (!browser) return { ...DEFAULTS };
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return { ...DEFAULTS };
		const parsed = JSON.parse(raw) as Partial<Prefs>;
		return {
			porn: isAdult(parsed.porn) ? parsed.porn : DEFAULTS.porn,
			sexual: isAdult(parsed.sexual) ? parsed.sexual : DEFAULTS.sexual,
			nudity: isAdult(parsed.nudity) ? parsed.nudity : DEFAULTS.nudity,
			graphicMedia: isAdult(parsed.graphicMedia) ? parsed.graphicMedia : DEFAULTS.graphicMedia,
			aiGenerated: isAi(parsed.aiGenerated) ? parsed.aiGenerated : DEFAULTS.aiGenerated
		};
	} catch {
		return { ...DEFAULTS };
	}
}

/**
 * Reactive viewer preferences for moderation rendering. Consumed by
 * <LabeledMedia> (blur + badge) and the upstream filter via shouldHide().
 */
export const modPrefs = $state<Prefs>(load());

function persist() {
	if (!browser) return;
	try {
		localStorage.setItem(
			STORAGE_KEY,
			JSON.stringify({
				porn: modPrefs.porn,
				sexual: modPrefs.sexual,
				nudity: modPrefs.nudity,
				graphicMedia: modPrefs.graphicMedia,
				aiGenerated: modPrefs.aiGenerated
			})
		);
	} catch {
		// quota or disabled storage; preferences remain in-memory for the session
	}
}

export function setAdult(key: AdultKey, val: AdultVisibility) {
	modPrefs[key] = val;
	persist();
}

export function setAi(val: AiVisibility) {
	modPrefs.aiGenerated = val;
	persist();
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
export function effectiveVisibility(
	labels?: LabelView[]
): 'show' | 'blur' | 'hide' {
	if (!labels || labels.length === 0) return 'show';
	let acc: 'show' | 'blur' | 'hide' = 'show';
	for (const l of labels) {
		acc = mostRestrictive(acc, visibilityFor(l.val));
		if (acc === 'hide') break;
	}
	return acc;
}

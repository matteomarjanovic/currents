import type { SaveView } from '$lib/types';

// The list arithmetic behind the detail view's swipe run. Kept apart from the reactive
// state in save-sequence.svelte.ts so it can be unit-tested without the Svelte runtime.

/** The save `offset` places from `uri`, or null at either end / when `uri` isn't in the run. */
export function neighbourOf(run: SaveView[], uri: string, offset: number): SaveView | null {
	const i = run.findIndex((s) => s.uri === uri);
	if (i === -1) return null;
	return run[i + offset] ?? null;
}

/** How many saves follow `uri` in the run, or -1 when it isn't part of one. */
export function countAfter(run: SaveView[], uri: string): number {
	const i = run.findIndex((s) => s.uri === uri);
	return i === -1 ? -1 : run.length - 1 - i;
}

/**
 * `run` plus everything in `incoming` it doesn't already hold, keeping incoming's order.
 * Returns `run` itself when there's nothing new, so a caller assigning the result to
 * reactive state doesn't churn on a grid that reported the same items again.
 */
export function appendUnseen(run: SaveView[], incoming: SaveView[]): SaveView[] {
	const seen = new Set(run.map((s) => s.uri));
	const fresh = incoming.filter((s) => !seen.has(s.uri));
	return fresh.length ? [...run, ...fresh] : run;
}

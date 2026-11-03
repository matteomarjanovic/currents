import type { SaveView } from '$lib/types';
import { SvelteSet } from 'svelte/reactivity';

interface FetchResult<T> {
	items: T[];
	cursor?: string;
	error?: unknown;
}

export function useInfiniteScroll<T = SaveView>(
	fetchFn: (cursor?: string) => Promise<FetchResult<T>>,
	getKey: (item: T) => string = (item) => (item as { uri?: string }).uri ?? '',
	initial?: FetchResult<T>
) {
	let items: T[] = $state(initial?.items ?? []);
	let cursor: string | undefined = $state(initial?.cursor);
	let loading = $state(false);
	let hasMore = $state(initial ? !!initial.cursor : true);
	// The error thrown by the last failed fetch (fetchers that swallow failures
	// and return an empty page never set it). While set, loadMore() is a no-op —
	// the scroll sentinel would otherwise retry in a loop — until retry()/reset().
	let error = $state<unknown>(initial?.error ?? null);
	let generation = 0;

	async function loadMore() {
		if (loading || !hasMore || error) return;
		loading = true;
		const requestGeneration = generation;
		try {
			const result = await fetchFn(cursor);
			if (requestGeneration !== generation) return;
			const seen = new SvelteSet(items.map(getKey));
			const fresh = result.items.filter((i) => !seen.has(getKey(i)));
			items = [...items, ...fresh];
			cursor = result.cursor;
			hasMore = !!result.cursor;
		} catch (e) {
			if (requestGeneration === generation) error = e;
		} finally {
			if (requestGeneration === generation) loading = false;
		}
	}

	function retry() {
		error = null;
		void loadMore();
	}

	function reset(next?: FetchResult<T>) {
		generation += 1;
		items = next?.items ?? [];
		cursor = next?.cursor;
		hasMore = next ? !!next.cursor : true;
		loading = false;
		error = next?.error ?? null;
	}

	function removeItem(key: string) {
		items = items.filter((i) => getKey(i) !== key);
	}

	return {
		get items() {
			return items;
		},
		get loading() {
			return loading;
		},
		get cursor() {
			return cursor;
		},
		get hasMore() {
			return hasMore;
		},
		get error() {
			return error;
		},
		loadMore,
		retry,
		reset,
		removeItem
	};
}

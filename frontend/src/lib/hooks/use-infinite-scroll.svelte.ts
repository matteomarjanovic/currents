import type { SaveView } from '$lib/types';

interface FetchResult<T> {
	items: T[];
	cursor?: string;
}

export function useInfiniteScroll<T = SaveView>(
	fetchFn: (cursor?: string) => Promise<FetchResult<T>>,
	getKey: (item: T) => string = (item) => (item as { uri?: string }).uri ?? ''
) {
	let items: T[] = $state([]);
	let cursor: string | undefined = $state(undefined);
	let loading = $state(false);
	let hasMore = $state(true);
	// The error thrown by the last failed fetch (fetchers that swallow failures
	// and return an empty page never set it). While set, loadMore() is a no-op —
	// the scroll sentinel would otherwise retry in a loop — until retry()/reset().
	let error = $state<unknown>(null);

	async function loadMore() {
		if (loading || !hasMore || error) return;
		loading = true;
		try {
			const result = await fetchFn(cursor);
			const seen = new Set(items.map(getKey));
			const fresh = result.items.filter((i) => !seen.has(getKey(i)));
			items = [...items, ...fresh];
			cursor = result.cursor;
			hasMore = !!result.cursor;
		} catch (e) {
			error = e;
		}
		loading = false;
	}

	function retry() {
		error = null;
		void loadMore();
	}

	function reset() {
		items = [];
		cursor = undefined;
		hasMore = true;
		loading = false;
		error = null;
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

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

	async function loadMore() {
		if (loading || !hasMore) return;
		loading = true;
		try {
			const result = await fetchFn(cursor);
			const seen = new Set(items.map(getKey));
			const fresh = result.items.filter((i) => !seen.has(getKey(i)));
			items = [...items, ...fresh];
			cursor = result.cursor;
			hasMore = !!result.cursor;
		} catch {
			hasMore = false;
		}
		loading = false;
	}

	function reset() {
		items = [];
		cursor = undefined;
		hasMore = true;
		loading = false;
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
		loadMore,
		reset
	};
}

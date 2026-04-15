import type { SaveView } from '$lib/types';

interface FetchResult {
	items: SaveView[];
	cursor?: string;
}

export function useInfiniteScroll(fetchFn: (cursor?: string) => Promise<FetchResult>) {
	let items: SaveView[] = $state([]);
	let cursor: string | undefined = $state(undefined);
	let loading = $state(false);
	let hasMore = $state(true);

	async function loadMore() {
		if (loading || !hasMore) return;
		loading = true;
		try {
			const result = await fetchFn(cursor);
			const seen = new Set(items.map((i) => i.uri));
			const fresh = result.items.filter((i) => !seen.has(i.uri));
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

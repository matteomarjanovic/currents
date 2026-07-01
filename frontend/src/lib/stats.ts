export type Granularity = 'day' | 'week' | 'month';
export type DailyPoint = { date: Date; count: number };
export type CumulativePoint = { date: Date; total: number };

// Truncate a date to the start of its UTC day / week (Monday) / month bucket.
function truncate(d: Date, g: Granularity): Date {
	if (g === 'month') return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), 1));
	const midnight = new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate()));
	if (g === 'day') return midnight;
	const monday = (midnight.getUTCDay() + 6) % 7; // Mon=0 … Sun=6
	midnight.setUTCDate(midnight.getUTCDate() - monday);
	return midnight;
}

function nextBucket(d: Date, g: Granularity): Date {
	if (g === 'month') return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 1));
	const step = g === 'week' ? 7 : 1;
	return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate() + step));
}

// Aggregate a daily series into contiguous buckets at the given granularity,
// filling empty buckets with 0 so bars/lines stay continuous. Assumes `points`
// is sorted oldest-first.
export function bucketize(points: DailyPoint[], g: Granularity): DailyPoint[] {
	if (points.length === 0) return [];
	const sums = new Map<number, number>();
	for (const p of points) {
		const k = truncate(p.date, g).getTime();
		sums.set(k, (sums.get(k) ?? 0) + p.count);
	}
	const end = truncate(points[points.length - 1].date, g);
	const out: DailyPoint[] = [];
	let cur = truncate(points[0].date, g);
	while (cur.getTime() <= end.getTime()) {
		out.push({ date: new Date(cur), count: sums.get(cur.getTime()) ?? 0 });
		cur = nextBucket(cur, g);
	}
	return out;
}

// Cumulative running total of registrations across the bucketed series.
export function cumulative(points: DailyPoint[], g: Granularity): CumulativePoint[] {
	let running = 0;
	return bucketize(points, g).map((b) => {
		running += b.count;
		return { date: b.date, total: running };
	});
}

<script lang="ts">
	import { onMount } from 'svelte';
	import { apiFetch } from '$lib/api';
	import * as Chart from '$lib/components/ui/chart';
	import * as Card from '$lib/components/ui/card';
	import * as Select from '$lib/components/ui/select';
	import { scaleUtc } from 'd3-scale';
	import { BarChart, LineChart, Highlight } from 'layerchart';
	import { curveNatural } from 'd3-shape';
	import { cubicInOut } from 'svelte/easing';
	import { bucketize, cumulative, type Granularity, type DailyPoint } from '$lib/stats';

	let totalUsers = $state(0);
	let daily = $state<DailyPoint[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let barGranularity = $state<Granularity>('week');
	let lineGranularity = $state<Granularity>('week');

	const granularities: { value: Granularity; label: string }[] = [
		{ value: 'day', label: 'Daily' },
		{ value: 'week', label: 'Weekly' },
		{ value: 'month', label: 'Monthly' }
	];

	function granularityLabel(g: Granularity): string {
		return granularities.find((x) => x.value === g)?.label ?? '';
	}

	onMount(async () => {
		try {
			const res = await apiFetch('/api/admin/stats');
			if (!res.ok) {
				error = `Failed to load stats (${res.status})`;
				return;
			}
			const data = (await res.json()) as {
				totalUsers: number;
				registrations: { date: string; count: number }[];
			};
			totalUsers = data.totalUsers;
			daily = (data.registrations ?? []).map((r) => ({ date: new Date(r.date), count: r.count }));
		} catch (e) {
			error = String(e);
		} finally {
			loading = false;
		}
	});

	const barData = $derived(bucketize(daily, barGranularity));
	const lineData = $derived(cumulative(daily, lineGranularity));

	function formatTick(d: Date, g: Granularity): string {
		if (g === 'month')
			return d.toLocaleDateString('en-US', { month: 'short', year: 'numeric', timeZone: 'UTC' });
		return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', timeZone: 'UTC' });
	}

	function formatFullDate(d: Date, g: Granularity): string {
		if (g === 'month')
			return d.toLocaleDateString('en-US', { month: 'long', year: 'numeric', timeZone: 'UTC' });
		const base = d.toLocaleDateString('en-US', {
			month: 'short',
			day: 'numeric',
			year: 'numeric',
			timeZone: 'UTC'
		});
		return g === 'week' ? `Week of ${base}` : base;
	}

	const firstDate = $derived(daily.length ? daily[0].date : null);

	const barConfig = {
		count: { label: 'New users', color: 'var(--chart-1)' }
	} satisfies Chart.ChartConfig;

	const lineConfig = {
		total: { label: 'Total users', color: 'var(--chart-1)' }
	} satisfies Chart.ChartConfig;
</script>

<svelte:head>
	<title>Stats · Currents</title>
</svelte:head>

<div class="flex flex-col gap-6">
	<header>
		<h1 class="text-xl font-semibold tracking-tight">Stats</h1>
		<p class="text-sm text-muted-foreground">User registration analytics.</p>
	</header>

	{#if loading}
		<div class="py-10 text-center text-sm text-muted-foreground">Loading…</div>
	{:else if error}
		<div class="rounded-md border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive">
			{error}
		</div>
	{:else}
		<Card.Root>
			<Card.Header>
				<Card.Description>Registered users</Card.Description>
				<Card.Title class="text-4xl tabular-nums">{totalUsers.toLocaleString()}</Card.Title>
			</Card.Header>
			{#if firstDate}
				<Card.Content>
					<p class="text-sm text-muted-foreground">
						Since {firstDate.toLocaleDateString('en-US', {
							month: 'long',
							day: 'numeric',
							year: 'numeric',
							timeZone: 'UTC'
						})}
					</p>
				</Card.Content>
			{/if}
		</Card.Root>

		<Card.Root>
			<Card.Header class="flex items-center gap-2 space-y-0 border-b py-5 sm:flex-row">
				<div class="grid flex-1 gap-1 text-center sm:text-start">
					<Card.Title>New registrations</Card.Title>
					<Card.Description>New users per {barGranularity}.</Card.Description>
				</div>
				<Select.Root type="single" bind:value={barGranularity}>
					<Select.Trigger class="w-36 rounded-lg sm:ms-auto" aria-label="Select granularity">
						{granularityLabel(barGranularity)}
					</Select.Trigger>
					<Select.Content class="rounded-xl">
						{#each granularities as g (g.value)}
							<Select.Item value={g.value} label={g.label} class="rounded-lg">{g.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</Card.Header>
			<Card.Content class="px-2 sm:p-6">
				{#if barData.length === 0}
					<div class="py-10 text-center text-sm text-muted-foreground">No data yet.</div>
				{:else}
					<Chart.Container config={barConfig} class="aspect-auto h-[260px] w-full">
						<BarChart
							data={barData}
							x="date"
							axis="x"
							series={[{ key: 'count', label: 'New users', color: barConfig.count.color }]}
							props={{
								bars: {
									stroke: 'none',
									rounded: 'none',
									motion: { type: 'tween', duration: 500, easing: cubicInOut }
								},
								highlight: { area: { fill: 'none' } },
								xAxis: {
									format: (d: Date) => formatTick(d, barGranularity),
									// The bar x-axis is a band scale, so ticks must be actual bucket
									// values (not d3-generated time ticks, which land off-band → NaN).
									// Thin to ~8 evenly spaced labels to avoid crowding.
									ticks: (scale) => {
										const domain = scale.domain() as Date[];
										const step = Math.max(1, Math.ceil(domain.length / 8));
										return domain.filter((_, i) => i % step === 0);
									}
								}
							}}
						>
							{#snippet belowMarks()}
								<Highlight area={{ class: 'fill-muted' }} />
							{/snippet}
							{#snippet tooltip()}
								<Chart.Tooltip labelFormatter={(v: Date) => formatFullDate(v, barGranularity)} />
							{/snippet}
						</BarChart>
					</Chart.Container>
				{/if}
			</Card.Content>
		</Card.Root>

		<Card.Root>
			<Card.Header class="flex items-center gap-2 space-y-0 border-b py-5 sm:flex-row">
				<div class="grid flex-1 gap-1 text-center sm:text-start">
					<Card.Title>Total users over time</Card.Title>
					<Card.Description>Cumulative registered users.</Card.Description>
				</div>
				<Select.Root type="single" bind:value={lineGranularity}>
					<Select.Trigger class="w-36 rounded-lg sm:ms-auto" aria-label="Select granularity">
						{granularityLabel(lineGranularity)}
					</Select.Trigger>
					<Select.Content class="rounded-xl">
						{#each granularities as g (g.value)}
							<Select.Item value={g.value} label={g.label} class="rounded-lg">{g.label}</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
			</Card.Header>
			<Card.Content class="px-2 sm:p-6">
				{#if lineData.length === 0}
					<div class="py-10 text-center text-sm text-muted-foreground">No data yet.</div>
				{:else}
					<Chart.Container config={lineConfig} class="aspect-auto h-[260px] w-full">
						<LineChart
							data={lineData}
							x="date"
							xScale={scaleUtc()}
							axis="x"
							series={[{ key: 'total', label: 'Total users', color: lineConfig.total.color }]}
							props={{
								spline: { curve: curveNatural, motion: 'tween', strokeWidth: 2 },
								xAxis: { format: (v: Date) => formatTick(v, lineGranularity) },
								highlight: { points: { r: 4 } }
							}}
						>
							{#snippet tooltip()}
								<Chart.Tooltip labelFormatter={(v: Date) => formatFullDate(v, lineGranularity)} />
							{/snippet}
						</LineChart>
					</Chart.Container>
				{/if}
			</Card.Content>
		</Card.Root>
	{/if}
</div>

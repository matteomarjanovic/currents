<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { Button } from '$lib/components/ui/button';
	import * as Popover from '$lib/components/ui/popover';
	import { Slider } from '$lib/components/ui/slider';
	import FlowField from './flow-field.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { loginPrompt } from '$lib/stores/login-prompt.svelte';

	const MIN = -0.6;
	const MAX = 1;
	const DEFAULT_VALUE = 0.6;
	const NOISE_AT_MIN = 7;
	const NOISE_AT_MAX = 0.5;

	function clamp(v: number): number {
		return Math.max(MIN, Math.min(MAX, v));
	}

	function valueToNoise(v: number): number {
		const t = (MAX - v) / (MAX - MIN);
		return NOISE_AT_MAX + t * (NOISE_AT_MIN - NOISE_AT_MAX);
	}

	const selectedValue = $derived.by(() => {
		const raw = page.url.searchParams.get('personalization');
		if (raw === null) return DEFAULT_VALUE;
		const n = Number(raw);
		return Number.isFinite(n) ? clamp(n) : DEFAULT_VALUE;
	});

	function buildExploreHref(v: number) {
		const query = [
			...Array.from(page.url.searchParams.entries()).filter(([key]) => key !== 'personalization'),
			['personalization', String(v)]
		]
			.map(([key, value]) => `${encodeURIComponent(key)}=${encodeURIComponent(value)}`)
			.join('&');

		return `/(with-navbar)/explore?${query}` as const;
	}

	let activeValue = $state(DEFAULT_VALUE);
	let committedValue = $state<number | null>(null);
	let open = $state(false);
	let previousOpen = false;

	$effect(() => {
		if (committedValue !== null && selectedValue === committedValue) committedValue = null;
	});

	$effect(() => {
		if (open !== previousOpen) {
			previousOpen = open;
			activeValue = committedValue ?? selectedValue;
			return;
		}

		if (!open) activeValue = committedValue ?? selectedValue;
	});

	const currentNoiseIntensity = $derived(
		valueToNoise(open ? activeValue : (committedValue ?? selectedValue))
	);

	async function commitValue(v: number) {
		const nextValue = clamp(v);
		const target = resolve(buildExploreHref(nextValue));

		committedValue = nextValue;
		activeValue = nextValue;
		open = false;

		try {
			await goto(target, { replaceState: true, keepFocus: true, noScroll: true });
		} catch {
			committedValue = null;
			activeValue = selectedValue;
		}
	}
</script>

<div
	role="group"
	aria-label="Personalization selector"
	class="relative inline-flex flex-col items-end select-none"
>
	<Popover.Root bind:open>
		<Popover.Trigger>
			{#snippet child({ props })}
				<Button
					{...props}
					variant="default"
					size="icon-lg"
					class="z-20 h-15 w-15 cursor-pointer overflow-hidden rounded-full border-none bg-primary-foreground/80 p-0 backdrop-blur-sm hover:bg-primary-foreground/80 aria-expanded:scale-95 aria-expanded:bg-primary-foreground/80"
					aria-label="Adjust personalization"
					onclick={(e: MouseEvent) => {
						if (!auth.user) {
							e.preventDefault();
							e.stopPropagation();
							loginPrompt.open = true;
							return;
						}

						(props.onclick as ((event: MouseEvent) => void) | undefined)?.(e);
					}}
				>
					<FlowField noiseIntensity={currentNoiseIntensity} />
				</Button>
			{/snippet}
		</Popover.Trigger>

		<Popover.Content
			side="top"
			align="end"
			sideOffset={8}
			class="w-56 items-center gap-2 bg-popover/70 px-4 py-3 backdrop-blur-2xl backdrop-saturate-150"
		>
			<span class="text-xs text-muted-foreground">Personalization level</span>
			<div
				class="flex w-full justify-between text-sm font-normal whitespace-nowrap text-popover-foreground"
			>
				<span>Personal</span>
				<span>New worlds</span>
			</div>
			<Slider
				type="single"
				bind:value={activeValue}
				min={MIN}
				max={MAX}
				step={0.01}
				dir="rtl"
				class="personalization-slider"
				onValueCommit={(v) => commitValue(v as number)}
				aria-label="Personalization level"
			/>
		</Popover.Content>
	</Popover.Root>
</div>


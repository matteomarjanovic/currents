<script lang="ts">
	import {
		modPrefs,
		modPrefsLoaded,
		loadModerationPrefs,
		setAdult,
		setAi,
		type AdultKey,
		type AdultVisibility,
		type AiVisibility
	} from '$lib/stores/moderation-prefs.svelte';
	import { onMount } from 'svelte';

	// Ensure prefs are loaded even if the user lands here before the top-bar's
	// load fires; gating the controls on this avoids overwriting saved prefs with
	// the defaults shown mid-load.
	onMount(() => {
		if (!modPrefsLoaded.value) void loadModerationPrefs();
	});

	const ADULT_CATEGORIES: { key: AdultKey; label: string; description: string }[] = [
		{
			key: 'porn',
			label: 'Porn',
			description: 'Sexually explicit imagery.'
		},
		{
			key: 'sexual',
			label: 'Sexual',
			description: 'Sexually suggestive content that stops short of explicit.'
		},
		{
			key: 'nudity',
			label: 'Nudity',
			description: 'Non-sexual nudity (artistic, documentary, fashion).'
		},
		{
			key: 'graphicMedia',
			label: 'Graphic violence',
			description: 'Gore, injury, or other distressing imagery.'
		}
	];

	const ADULT_OPTIONS: { val: AdultVisibility; label: string }[] = [
		{ val: 'show', label: 'Show' },
		{ val: 'blur', label: 'Blur' },
		{ val: 'hide', label: 'Hide' }
	];

	const AI_OPTIONS: { val: AiVisibility; label: string }[] = [
		{ val: 'show', label: 'Show' },
		{ val: 'hide', label: 'Hide' }
	];
</script>

<svelte:head>
	<title>Settings · Currents</title>
</svelte:head>

<div class="mx-auto mb-12 flex w-full max-w-2xl flex-col gap-8">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">Settings</h1>
		<p class="text-sm text-muted-foreground">
			Synced to your account across devices. <b>Hide</b> filters matching saves out of every feed
			entirely. <b>Blur</b> shows a click-to-reveal warning. <b>Show</b> renders normally.
		</p>
	</header>

	<section class="flex flex-col gap-4">
		<div>
			<h2 class="text-lg font-medium">Adult content</h2>
			<p class="text-sm text-muted-foreground">Set how each label is treated independently.</p>
		</div>
		<div class="flex flex-col gap-3">
			{#each ADULT_CATEGORIES as cat (cat.key)}
				<div
					class="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 sm:flex-row sm:items-center sm:justify-between"
				>
					<div class="flex flex-col gap-0.5">
						<span class="text-sm font-medium">{cat.label}</span>
						<span class="text-xs text-muted-foreground">{cat.description}</span>
					</div>
					<div
						class="inline-flex shrink-0 self-start rounded-md border border-border p-0.5 sm:self-auto"
					>
						{#each ADULT_OPTIONS as opt (opt.val)}
							{@const active = modPrefs[cat.key] === opt.val}
							<button
								type="button"
								disabled={!modPrefsLoaded.value}
								onclick={() => setAdult(cat.key, opt.val)}
								class="rounded px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50 {active
									? 'bg-foreground text-background'
									: 'text-muted-foreground hover:bg-muted'}"
							>
								{opt.label}
							</button>
						{/each}
					</div>
				</div>
			{/each}
		</div>
	</section>

	<section class="flex flex-col gap-4">
		<div>
			<h2 class="text-lg font-medium">AI-generated content</h2>
			<p class="text-sm text-muted-foreground">
				How should images detected as AI-generated appear?
			</p>
		</div>
		<div
			class="flex flex-col gap-3 rounded-lg border border-border bg-card p-4 sm:flex-row sm:items-center sm:justify-between"
		>
			<div class="flex flex-col gap-0.5">
				<span class="text-sm font-medium">AI-generated</span>
				<span class="text-xs text-muted-foreground">
					When shown, a small "AI" badge appears in the corner.
				</span>
			</div>
			<div
				class="inline-flex shrink-0 self-start rounded-md border border-border p-0.5 sm:self-auto"
			>
				{#each AI_OPTIONS as opt (opt.val)}
					{@const active = modPrefs.aiGenerated === opt.val}
					<button
						type="button"
						disabled={!modPrefsLoaded.value}
						onclick={() => setAi(opt.val)}
						class="rounded px-3 py-1.5 text-xs font-medium transition-colors disabled:opacity-50 {active
							? 'bg-foreground text-background'
							: 'text-muted-foreground hover:bg-muted'}"
					>
						{opt.label}
					</button>
				{/each}
			</div>
		</div>
	</section>
</div>

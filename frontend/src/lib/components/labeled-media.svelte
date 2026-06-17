<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { LabelView } from '$lib/types';
	import { effectiveVisibility, visibilityFor } from '$lib/stores/moderation-prefs.svelte';
	import EyeOff from '@lucide/svelte/icons/eye-off';
	import Sparkles from '@lucide/svelte/icons/sparkles';

	interface Props {
		labels?: LabelView[];
		children: Snippet;
		/**
		 * Optional overlay rendered above the image but below the blur overlay
		 * and AI badge. Use this for hover UI (e.g. save buttons) so it stays
		 * within the same stacking context and is correctly obscured by the blur
		 * overlay when content is flagged.
		 */
		overlay?: Snippet;
		/**
		 * Tailwind classes applied to BOTH the outer wrapper and the inner
		 * blur-target div. Use this to propagate sizing constraints so a child
		 * <img class="max-h-full max-w-full object-contain"> sizes against the
		 * consumer's layout, not the wrapper's natural content size.
		 *
		 * Heads-up: when the wrapper sits inside a flex parent with
		 * `items-center` / `items-start` etc. (i.e. NOT `items-stretch`), the
		 * wrapper does not inherit the parent's height by default, so `max-h-full`
		 * down the chain resolves to "none". Pass an explicit `h-full w-full`
		 * (or equivalent) along with `flex items-center justify-center` to give
		 * the wrapper a definite size for the img's max-* constraints to bind to.
		 */
		class?: string;
	}

	let { labels, children, overlay, class: className = '' }: Props = $props();

	// Labels whose presence may trigger a click-to-reveal blur overlay. Used
	// purely for picking the human-readable warning text in `summarize()`.
	// Whether a blur actually fires is decided by `effectiveVisibility()` —
	// which consults the viewer's per-label preferences.
	const BLUR_LABELS = new Set(['porn', 'nudity', 'sexual', 'graphic-media']);
	const AI_VALS = new Set(['currents-ai-generated']);

	let labelList = $derived(labels ?? []);
	let effective = $derived(effectiveVisibility(labelList));
	let blurMatches = $derived(labelList.filter((l) => BLUR_LABELS.has(l.val)));

	let revealed = $state(false);
	let shouldBlur = $derived(effective === 'blur' && !revealed);

	// AI badge: render the corner badge when the save has any AI label AND the
	// AI visibility resolved to "show", as long as it wouldn't sit behind the blur
	// overlay — i.e. the image isn't blurred, or it has been revealed. (A save can
	// be both AI-generated and, say, sexual; its AI provenance should still show
	// once the image is unblurred.)
	let aiBadge = $derived(
		labelList.some((l) => AI_VALS.has(l.val)) &&
			visibilityFor('currents-ai-generated') === 'show' &&
			!shouldBlur
	);

	function summarize(): string {
		const present = new Set(blurMatches.map((l) => l.val));
		if (present.has('porn')) return 'Sexually explicit';
		if (present.has('sexual')) return 'Sexual content';
		if (present.has('nudity')) return 'Nudity';
		if (present.has('graphic-media')) return 'Violent content';
		return 'Sensitive content';
	}

	function reveal(e: MouseEvent) {
		e.preventDefault();
		e.stopPropagation();
		revealed = true;
	}
</script>

<!--
  effective === 'hide' should not normally reach this component — MasonryGrid
  and save-detail filter upstream. We bail out silently as defense in depth
  rather than rendering a placeholder.
-->
{#if effective !== 'hide'}
	<div class="relative isolate {className}">
		<div
			class="transition-[filter] duration-200 {className}"
			class:blur-2xl={shouldBlur}
			class:pointer-events-none={shouldBlur}
			class:select-none={shouldBlur}
		>
			{@render children()}
		</div>

		{#if overlay && !shouldBlur}
			{@render overlay()}
		{/if}

		{#if shouldBlur}
			<div
				class="absolute inset-0 z-50 flex flex-col items-center justify-center bg-black/30 p-3 text-center text-white backdrop-blur-sm"
			>
				<EyeOff class="size-6" />
				<p class="text-sm font-medium">{summarize()}</p>
				<a
					href="/settings"
					class="pb-5 text-xs text-white/80 underline underline-offset-2 transition-colors hover:text-white"
				>
					Change visibility settings
				</a>
				<button
					type="button"
					class="rounded-md bg-white px-3 py-1.5 text-xs font-medium text-black transition-colors hover:bg-white/90"
					onclick={reveal}
				>
					Show
				</button>
			</div>
		{/if}

		{#if aiBadge}
			<div
				class="pointer-events-none absolute top-2 right-2 z-10 inline-flex items-center gap-1 rounded-full bg-black/60 px-2 py-0.5 text-xs text-white backdrop-blur-sm"
				title="AI-generated"
			>
				<Sparkles class="size-3" aria-hidden="true" />
				<span>AI</span>
			</div>
		{/if}
	</div>
{/if}

<script lang="ts" module>
	export const references = {
		'blue-water': 'Cobalt ripples on the surface of water',
		'pale-stairs': 'A pale staircase framed by a geometric opening',
		poppies: 'A red poppy against a dark green background',
		'misty-forest': 'Mist hanging over a dark evergreen forest',
		'sunlit-chair': 'A linen armchair beside sheer curtains',
		'indigo-silk': 'Folds in dark blue fabric',
		celadon: 'A green ceramic cup in soft window light',
		'color-study': 'Colored lines forming an angular pattern against black',
		'sunlit-sofa': 'A low sofa in a sunlit room',
		'red-drapery': 'Vertical folds of orange and vermilion fabric',
		'blue-pier': 'A long pier over still water at blue hour',
		fern: 'A green fern frond against a dark background',
		letterpress: 'Wooden letterpress type seen at an angle',
		'violet-ridge': 'A mountain ridge under a violet sky',
		'pottery-wheel': 'Hands shaping a pale clay pot on a wheel',
		'sand-ripples': 'Footprints across ripples in pale sand dunes',
		persimmons: 'Persimmons arranged on a dark plate',
		'woven-textile': 'Close-up of crinkled orange fabric',
		'lit-moss': 'A patch of moss and woodland leaves in golden light',
		'shore-pebbles': 'Rounded gray-blue stones on a shore',
		'spiral-stair': 'A black and white photograph looking down a spiral staircase',
		'golden-facade': 'A sunlit stone facade framed by buildings and blue sky',
		'glass-sphere': 'A glass sphere on a rough surface against a blue sky',
		'pink-anemone': 'A pink anemone against a black background',
		'amber-study': 'An amber vessel lit warmly from within',
		'frost-detail': 'A close-up of frost on a pale blue surface',
		'glazed-teapot': 'A dark glazed teapot in soft light',
		'blue-horizon': 'A deep blue horizon reflected in water at twilight',
		'sandstone-arch': 'A sandstone arch in a desert landscape at dusk',
		'cloud-sea': 'A pale sky above layers of white cloud',
		'concrete-facade': 'Repeating concrete forms on a building facade',
		'sea-steps': 'Railings leading into hazy water',
		'studio-textiles': 'Layers of brightly colored cloth',
		'cut-grapefruit': 'Slices of pink grapefruit',
		'field-poppies': 'Red poppies against a bright blue sky',
		'window-seat': 'A chair and small table beside a tall window',
		'type-keys': 'The keys of an old typewriter in close-up',
		'cloud-hills': 'Low clouds wrapped around a mountain',
		'golden-woods': 'Sunlight falling between trees in a forest',
		'paper-fold': 'An orange origami boat on pale paper',
		'glass-marbles': 'Blue and green glass marbles on a white surface',
		'coastal-foam': 'White surf rolling onto a coast beneath a cliff',
		'red-door': 'A red building facade with a pale door',
		'blue-bottles': 'Blue glass vessels lined up against a window',
		'concrete-angles': 'Angular white concrete against the sky',
		'olive-silk': 'Folds and highlights in olive silk',
		'leaf-veins': 'A close-up of the pale veins of a green leaf',
		'dark-tunnel': 'A tunnel with repeating lines and warm brown shadows',
		'profile-flower': 'Two orange flowers in close-up',
		'quiet-armchair': 'A pale upholstered armchair beside dark branches and yellow flowers',
		'quiet-table': 'A wooden dining table and chairs beside a large window',
		'quiet-ceramic': 'A dark ceramic planter on a wooden table',
		'quiet-window': 'A sunlit wooden surface beneath two windows',
		'afternoon-room': 'Afternoon light in a room with a pale wooden floor',
		'afternoon-table': 'A long wooden table in a room with warm window light',
		'indigo-building': 'Blue glass towers viewed from below',
		'indigo-tiles': 'Irregular deep blue ceramic mosaic tiles',
		'indigo-curtain': 'A dark curtain lit with saturated blue light',
		'red-grating': 'Red metal grating crossed by a dark chain',
		'red-lantern': 'A red Chinese lantern seed pod against a dark background',
		'red-lights': 'Rows of red lights receding into darkness',
		'moss-sprigs': 'Closely spaced green evergreen sprigs',
		'moss-carpet': 'A shallow-focus carpet of green moss',
		'moss-glass': 'Green glass bottles with pale green flowers',
		'related-fern': 'Overlapping dark green fern fronds',
		'related-pattern': 'Pale fern fronds against a deep teal background',
		'related-fiddlehead': 'A curled green fiddlehead in close-up',
		'feed-stair': 'A golden spiral staircase viewed from above',
		'feed-blossoms': 'Pink blossoms against a pale blue sky',
		'feed-coast': 'White water washing over rocks at dusk',
		'feed-chair': 'A dark armchair and sideboard in a quiet room',
		'feed-bedroom': 'A bedroom lit by corner windows',
		'feed-interior': 'A bright interior with wooden furniture',
		'feed-bowls': 'Small pottery bowls filled with spices',
		'feed-canyon': 'Curving strata in a sandstone canyon',
		'feed-lamps': 'Two warm pendant lights against a dark wall',
		'rhinoceros-print': 'A historical woodcut of a rhinoceros',
		'library-weave': 'A blue woven checkerboard pattern',
		'library-denim': 'A folded seam in blue denim'
	};
	export type Reference = keyof typeof references;
</script>

<script lang="ts">
	import { asset } from '$app/paths';

	let {
		id,
		decorative = false,
		eager = false,
		minViewportWidth,
		priority = false,
		class: className,
		style
	}: {
		id: Reference;
		decorative?: boolean;
		eager?: boolean;
		minViewportWidth?: number;
		priority?: boolean;
		class?: string;
		style?: string;
	} = $props();
	const source = $derived(asset(`landing/${id}.webp`));
</script>

{#snippet image(src: string)}
	<img
		{src}
		alt={decorative ? '' : references[id]}
		width="360"
		height="360"
		loading={eager ? 'eager' : 'lazy'}
		fetchpriority={priority ? 'high' : undefined}
		decoding="async"
		class={className}
		{style}
	/>
{/snippet}

{#if minViewportWidth}
	<picture>
		<source media="(width > {minViewportWidth}px)" srcset={source} />
		{@render image('data:image/gif;base64,R0lGODlhAQABAAD/ACwAAAAAAQABAAACADs=')}
	</picture>
{:else}
	{@render image(source)}
{/if}

<style>
	img {
		display: block;
		width: 100%;
		object-fit: cover;
		background: var(--muted);
	}
</style>

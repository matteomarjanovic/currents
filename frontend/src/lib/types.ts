// One collection-preview image plus the active label values on its blob, so the
// card can apply the viewer's blur/hide preferences (same logic as save tiles).
export interface PreviewItem {
	url: string;
	labels?: string[];
}

export interface CollectionView {
	uri: string;
	cid?: string;
	author?: {
		did: string;
		handle: string;
		displayName?: string;
		avatar?: string;
	};
	name: string;
	description?: string;
	parentUri?: string;
	saveCount?: number;
	favouriteCount?: number;
	previews?: PreviewItem[];
	createdAt?: string;
	lastSavedAt?: string;
	// `favourite` is the AT-URI of the viewer's favourite record (present iff favourited).
	viewer?: { favourite?: string };
}

export interface ActorProfileView {
	did: string;
	handle: string;
	displayName?: string;
	description?: string;
	pronouns?: string;
	website?: string;
	avatar?: string;
	banner?: string;
	createdAt?: string;
	followersCount?: number;
	followsCount?: number;
	viewer?: { following?: string; followedBy?: string };
}

export interface SaveAttribution {
	url?: string;
	license?: string;
	credit?: string;
}

export interface ImageContentView {
	$type: 'is.currents.content.defs#imageView';
	blobCid: string;
	imageUrl: string;
	width?: number;
	height?: number;
	dominantColor?: string;
	alt?: string;
	attribution?: SaveAttribution;
}

export type SaveContentView =
	| ImageContentView
	| {
			$type: string;
			[key: string]: unknown;
	  };

export interface LabelView {
	src: string;
	val: string;
	cts: string;
}

export interface SaveView {
	uri: string;
	author: {
		did: string;
		handle: string;
		displayName?: string;
		avatar?: string;
	};
	content: SaveContentView;
	text?: string;
	originUrl?: string;
	resaveOf?: { uri: string; cid: string };
	createdAt: string;
	viewer?: {
		saves?: { collectionUri: string; saveUri: string }[];
		attribution?: SaveAttribution;
		suspected?: boolean;
	};
	labels?: LabelView[];
}

export function isImageContentView(content: SaveContentView): content is ImageContentView {
	return content.$type === 'is.currents.content.defs#imageView';
}

export function getImageContent(save: Pick<SaveView, 'content'>): ImageContentView | null {
	return isImageContentView(save.content) ? save.content : null;
}

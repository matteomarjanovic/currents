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
	saveCount?: number;
	previewImages?: string[];
	createdAt?: string;
	viewer?: { starred?: boolean };
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
	attribution?: SaveAttribution;
}

export type SaveContentView =
	| ImageContentView
	| {
		$type: string;
		[key: string]: unknown;
	};

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
	viewer?: { saves?: { collectionUri: string; saveUri: string }[] };
}

export function isImageContentView(content: SaveContentView): content is ImageContentView {
	return content.$type === 'is.currents.content.defs#imageView';
}

export function getImageContent(save: Pick<SaveView, 'content'>): ImageContentView | null {
	return isImageContentView(save.content) ? save.content : null;
}
